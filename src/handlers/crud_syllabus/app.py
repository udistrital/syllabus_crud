# CRUD syllabus
# Get one, Get All, Post, Put and Delete endpoints

import json
import os
import uuid
from datetime import datetime
from typing import List, Dict, Optional, Any, Union

import pytz
from bson import ObjectId
from pydantic import BaseModel, Field, field_validator
from pymongo import MongoClient, ASCENDING, DESCENDING, errors

# Required environment variables
SYLLABUS_CRUD_HOST = os.environ.get('SYLLABUS_CRUD_HOST')
SYLLABUS_CRUD_PORT = os.environ.get('SYLLABUS_CRUD_PORT')
SYLLABUS_CRUD_USERNAME = os.environ.get('SYLLABUS_CRUD_USERNAME')
SYLLABUS_CRUD_PASS = os.environ.get('SYLLABUS_CRUD_PASS')
SYLLABUS_CRUD_DB = os.environ.get('SYLLABUS_CRUD_DB')
TIMEZONE = os.environ.get('TIMEZONE')
COLLECTION = "syllabus"

ORDER_LABEL = {
    "desc": DESCENDING,
    "asc": ASCENDING
}

# Estados del flujo de aprobación
ESTADO_ACTIVO = "activo"
ESTADO_ASIGNADO = "asignado"
ESTADO_EN_REVISION = "en_revision"
ESTADO_RECHAZADO = "rechazado"
ESTADO_EXPIRADO = "expirado"
ESTADO_CANCELADO = "cancelado"
ESTADO_APROBADO = "aprobado"

# Estados que corresponden a una propuesta no publicada (invisibles al público general)
ESTADOS_BORRADOR = {ESTADO_ASIGNADO, ESTADO_EN_REVISION, ESTADO_RECHAZADO, ESTADO_EXPIRADO}

# Máquina de estados: origen -> destinos permitidos
TRANSICIONES_ESTADO = {
    ESTADO_ASIGNADO: {ESTADO_ASIGNADO, ESTADO_EN_REVISION, ESTADO_EXPIRADO, ESTADO_CANCELADO},
    ESTADO_RECHAZADO: {ESTADO_ASIGNADO, ESTADO_EN_REVISION, ESTADO_CANCELADO},
    ESTADO_EXPIRADO: {ESTADO_ASIGNADO, ESTADO_CANCELADO},
    ESTADO_EN_REVISION: {ESTADO_RECHAZADO, ESTADO_CANCELADO},
    ESTADO_APROBADO: {ESTADO_ACTIVO},
    ESTADO_ACTIVO: set(),
    ESTADO_CANCELADO: set(),
}

# Campos que nunca deben modificarse mediante una actualización genérica de contenido
CAMPOS_PROTEGIDOS_UPDATE = {
    "_id", "syllabus_code", "version", "syllabus_actual", "estado", "creado_por", "flujo"
}

# Estados sobre los que no se permite actualizar contenido
ESTADOS_NO_EDITABLES_CONTENIDO = {ESTADO_EN_REVISION, ESTADO_CANCELADO}


def local_now():
    return datetime.now(tz=pytz.timezone(TIMEZONE))


def estado_documento(syllabus: dict) -> str:
    """Estado efectivo de un documento (retrocompatible: sin campo estado => activo)."""
    return syllabus.get("estado") or ESTADO_ACTIVO


def validar_transicion(origen: str, destino: str) -> bool:
    """Indica si una transición de estado está permitida por la máquina de estados."""
    return destino in TRANSICIONES_ESTADO.get(origen, set())


def build_historial_entry(estado: str, tercero_id=None, tercero_documento=None,
                          observacion=None) -> dict:
    """Construye una entrada de auditoría para el historial del flujo."""
    return {
        "estado": estado,
        "fecha": local_now().isoformat(),
        "tercero_id": tercero_id,
        "tercero_documento": tercero_documento,
        "observacion": observacion,
    }

# MODELOS AUXILIARES
class ObjetivoEspecifico(BaseModel):
    objetivo: str


class ResultadoDetallado(BaseModel):
    id: str  # Ej: "01", "02", "03" - ID único para referenciar en evaluación
    dominio: str  # Ej: "Cognitivo - Conocer", "Cognitivo - Analizar"
    resultado_detallado: str  # Descripción completa del resultado esperado


class CompetenciaCompleta(BaseModel):
    competencia: str  # Descripción general de la competencia
    resultados: list[ResultadoDetallado]  # Lista de resultados específicos


# MODELOS LEGACY COMPATIBLES
class ResultadoAprendizajeLegacy(BaseModel):
    pfa_programa: str | None = None
    pfa_asignatura: str | None = None
    competencias: str | None = None
    # Campos del nuevo formato
    competencia: str | None = None
    resultados: list[ResultadoDetallado] | None = None


class EstrategiaLegacy(BaseModel):
    descripcion: str


class EvaluacionLegacy(BaseModel):
    descripcion: str | None = None
    evaluaciones: list[dict] | None = None
    # Campos del nuevo formato
    tipos_evaluacion: list[dict] | None = None


class TipoEvaluacion(BaseModel):
    nombre: str  # Ej: "Actividades Entregables", "Talleres", "Parciales"
    tipo_evaluacion: str  # EF, EHP, EE
    porcentaje: int  # Peso porcentual (puede ser 0 para tipos inactivos)
    trabajo_tipo: str  # I (Individual), G (Grupal)
    tipo_nota: str = "0-5"  # Escala de calificación
    resultados_aprendizaje_asociados: list[str] = []  # IDs de resultados asociados


class EvaluacionNueva(BaseModel):
    tipos_evaluacion: list[TipoEvaluacion]
    
    @field_validator('tipos_evaluacion')
    def validate_porcentajes_activos(cls, v):
        """
        Valida que los porcentajes activos tengan sentido académico.
        Permite tipos con 0% (plantillas inactivas) pero advierte sobre inconsistencias.
        """
        total_activo = sum(tipo.porcentaje for tipo in v if tipo.porcentaje > 0)
        if total_activo > 0 and total_activo != 100:
            print(f"Advertencia: Los porcentajes activos suman {total_activo}%, no 100%")
        return v


class Bibliografia(BaseModel):
    basicas: Optional[List[str]] = []
    complementarias: Optional[List[str]] = []
    bases: Optional[List[str]] = []
    paginasWeb: Optional[List[str]] = []

    class Config:
        extra = "allow"  # Conserva campos de documentos historicos


class Seguimiento(BaseModel):
    elaboro: Optional[str] = None
    reviso: Optional[str] = None
    aprobo: Optional[str] = None
    fecha_elaboro: Optional[str] = None
    fechaRevisionConsejo: Optional[str] = None
    fechaAprobacionConsejo: Optional[str] = None
    numeroActa: Optional[str] = None
    archivo: Optional[str] = None

    class Config:
        extra = "allow"  # Conserva campos de documentos historicos


class HistorialEstado(BaseModel):
    estado: str
    fecha: Optional[str] = None
    tercero_id: Optional[int] = None
    tercero_documento: Optional[str] = None
    observacion: Optional[str] = None

    class Config:
        extra = "allow"  # Conserva campos adicionales


class Flujo(BaseModel):
    """Metadata del flujo de aprobación de un syllabus."""
    asignado_a: Optional[int] = None
    asignado_a_documento: Optional[str] = None
    asignado_por: Optional[int] = None
    asignado_por_documento: Optional[str] = None
    fecha_asignacion: Optional[str] = None
    fecha_limite: Optional[str] = None
    fecha_envio: Optional[str] = None
    # Snapshot de la última decisión del coordinador (rechazo, cancelación o
    # publicación). El detalle por evento vive en `historial`.
    fecha_ultima_decision: Optional[str] = None
    ultima_decision_por: Optional[int] = None
    motivo_rechazo: Optional[str] = None
    historial: Optional[List[HistorialEstado]] = []

    class Config:
        extra = "allow"  # Conserva campos adicionales


class SyllabusModel(BaseModel):
    """Modelo de datos del Syllabus"""
    syllabus_code: str | None = None
    version: int | None = 0
    syllabus_actual: bool | None = False
    estado: str = Field(default=ESTADO_ACTIVO)
    creado_por: Optional[int] = None
    flujo: Optional[Flujo] = None
    espacio_academico_id: int
    proyecto_curricular_ids: list[int] 
    plan_estudios_ids: list[int] 
    justificacion: str | None = None
    objetivo_general: str | None = None
    objetivos_especificos: list[ObjetivoEspecifico] | None = None
    resultados_aprendizaje: list[dict] | None = None  # Más flexible para ambos formatos
    articulacion_resultados_aprendizaje: str | None = None
    contenido: dict | None = None
    estrategias: dict[str, bool] | list[dict] | None = None  # Más flexible
    evaluacion: dict | None = None  # Más flexible para ambos formatos
    bibliografia: Bibliografia | None = None
    seguimiento: Seguimiento | None = None
    sugerencias: str | None = None
    recursos_educativos: str | None = None
    practicas_academicas: str | None = None
    vigencia: dict | None = None
    idioma_espacio_id: list | None = None
    tercero_id: int | None = 0
    activo: bool = Field(default=True)
    
    class Config:
        extra = "allow"  # Permite campos adicionales


class SyllabusCreationModel(SyllabusModel):
    fecha_creacion: datetime = Field(default=local_now())
    fecha_modificacion: datetime | None = None


class SyllabusUpdateModel(SyllabusModel):
    # Actualización parcial: los campos obligatorios en creación pasan a ser
    # opcionales para permitir un $set de únicamente los campos enviados.
    espacio_academico_id: int | None = None
    proyecto_curricular_ids: list[int] | None = None
    plan_estudios_ids: list[int] | None = None
    fecha_modificacion: datetime | None = Field(default=local_now())


class SyllabusTransitionModel(BaseModel):
    """Payload para cambiar el estado de un syllabus dentro del flujo de aprobación."""
    estado: str
    tercero_id: Optional[int] = None
    tercero_documento: Optional[str] = None
    observacion: Optional[str] = None
    fecha_limite: Optional[str] = None
    motivo_rechazo: Optional[str] = None
    asignado_a: Optional[int] = None
    asignado_a_documento: Optional[str] = None

    class Config:
        extra = "ignore"


class SyllabusPublishModel(BaseModel):
    """Payload para publicar (aprobar) una propuesta."""
    tercero_id: Optional[int] = None
    tercero_documento: Optional[str] = None
    nombre_aprobador: Optional[str] = None
    numeroActa: Optional[str] = None
    archivo: Optional[str] = None
    observacion: Optional[str] = None

    class Config:
        extra = "ignore"


class DeleteSyllabusModel(BaseModel):
    activo: bool | None = Field(default=False)
    fecha_modificacion: datetime | None = Field(default=local_now())


def transform_legacy_to_new_format(syllabus_data: dict) -> dict:
    """
    Transforma datos del formato legacy al nuevo formato
    """
    transformed_data = syllabus_data.copy()
    
    # Transformar resultados_aprendizaje
    if transformed_data.get("resultados_aprendizaje"):
        new_resultados = []
        for resultado in transformed_data["resultados_aprendizaje"]:
            if isinstance(resultado, dict):
                # Si tiene campos legacy, crear estructura nueva
                if "pfa_programa" in resultado or "pfa_asignatura" in resultado or "competencias" in resultado:
                    new_resultado = {
                        "competencia": resultado.get("pfa_programa", ""),
                        "resultados": [
                            {
                                "id": "01",
                                "dominio": "Cognitivo - Conocer",
                                "resultado_detallado": resultado.get("pfa_programa", "")
                            }
                        ]
                    }
                    new_resultados.append(new_resultado)
                else:
                    # Ya está en formato nuevo
                    new_resultados.append(resultado)
        transformed_data["resultados_aprendizaje"] = new_resultados
    
    # Transformar estrategias
    if transformed_data.get("estrategias") and isinstance(transformed_data["estrategias"], list):
        # Convertir lista de estrategias a diccionario
        estrategias_dict = {}
        for i, estrategia in enumerate(transformed_data["estrategias"]):
            if isinstance(estrategia, dict) and "descripcion" in estrategia:
                estrategias_dict[f"estrategia_{i+1}"] = True
        transformed_data["estrategias"] = estrategias_dict
    
    # Transformar evaluacion
    if transformed_data.get("evaluacion") and isinstance(transformed_data["evaluacion"], dict):
        evaluacion_data = transformed_data["evaluacion"]
        if "evaluaciones" in evaluacion_data and "tipos_evaluacion" not in evaluacion_data:
            # Convertir evaluaciones legacy a tipos_evaluacion
            tipos_evaluacion = []
            for i, eval_item in enumerate(evaluacion_data.get("evaluaciones", [])):
                if isinstance(eval_item, dict):
                    # Manejar porcentaje que puede ser None
                    porcentaje = eval_item.get("porcentaje")
                    if porcentaje is None:
                        porcentaje = 0
                    elif isinstance(porcentaje, str):
                        try:
                            porcentaje = int(porcentaje)
                        except (ValueError, TypeError):
                            porcentaje = 0
                    
                    tipo_eval = {
                        "nombre": eval_item.get("nombre", f"Evaluación {i+1}"),
                        "tipo_evaluacion": "EF",  # Default
                        "porcentaje": porcentaje,
                        "trabajo_tipo": "I",  # Default individual
                        "tipo_nota": "0-5",
                        "resultados_aprendizaje_asociados": []
                    }
                    tipos_evaluacion.append(tipo_eval)
            evaluacion_data["tipos_evaluacion"] = tipos_evaluacion
    
    return transformed_data


def transform_new_to_legacy_format(syllabus_data: dict) -> dict:
    """
    Transforma datos del nuevo formato al formato legacy
    """
    transformed_data = syllabus_data.copy()
    
    # Transformar resultados_aprendizaje
    if transformed_data.get("resultados_aprendizaje"):
        legacy_resultados = []
        for resultado in transformed_data["resultados_aprendizaje"]:
            if isinstance(resultado, dict):
                if "competencia" in resultado and "resultados" in resultado:
                    # Convertir de nuevo a legacy
                    legacy_resultado = {
                        "pfa_programa": resultado.get("competencia", ""),
                        "pfa_asignatura": "No aplica",
                        "competencias": "No aplica"
                    }
                    legacy_resultados.append(legacy_resultado)
                else:
                    # Ya está en formato legacy
                    legacy_resultados.append(resultado)
        transformed_data["resultados_aprendizaje"] = legacy_resultados
    
    # Transformar estrategias
    if transformed_data.get("estrategias") and isinstance(transformed_data["estrategias"], dict):
        # Convertir diccionario a lista de estrategias
        estrategias_list = []
        for key, value in transformed_data["estrategias"].items():
            if value:
                estrategias_list.append({"descripcion": f"Estrategia {key}"})
        transformed_data["estrategias"] = estrategias_list
    
    # Transformar evaluacion
    if transformed_data.get("evaluacion") and isinstance(transformed_data["evaluacion"], dict):
        evaluacion_data = transformed_data["evaluacion"]
        if "tipos_evaluacion" in evaluacion_data and "evaluaciones" not in evaluacion_data:
            # Convertir tipos_evaluacion a evaluaciones legacy
            evaluaciones = []
            for tipo_eval in evaluacion_data.get("tipos_evaluacion", []):
                if isinstance(tipo_eval, dict):
                    eval_item = {
                        "nombre": tipo_eval.get("nombre", ""),
                        "estrategia": "Evaluación",
                        "momento": "Según cronograma",
                        "porcentaje": tipo_eval.get("porcentaje", None)
                    }
                    evaluaciones.append(eval_item)
            evaluacion_data["evaluaciones"] = evaluaciones
    
    return transformed_data


def connect_db_client():
    """Genera el cliente para establecer la conexión con la base de datos"""
    try:
        # With password
        if SYLLABUS_CRUD_USERNAME and SYLLABUS_CRUD_PASS:
            uri = f"mongodb://{SYLLABUS_CRUD_USERNAME}:{SYLLABUS_CRUD_PASS}@" \
                  f"{SYLLABUS_CRUD_HOST}:{SYLLABUS_CRUD_PORT}/"
        else:
            # Without password
            uri = f"mongodb://{SYLLABUS_CRUD_HOST}:{SYLLABUS_CRUD_PORT}/"

        client = MongoClient(uri, uuidRepresentation='standard')
        print("Client DB Successful")
        return client
    except Exception as ex:
        print("Error Client DB")
        print(f"Detail: {ex}")
        return None


def close_connect_db(client):
    """Permite cerrar la conexión a la base de datos en caso que la conexión exista"""
    try:
        print("Closing client DB")
        if client:
            client.close()
    except Exception as ex:
        print("Error close Client DB")
        print(f"Detail: {ex}")


def parse_body(event) -> tuple:
    """
    Deserialización de parámetros de entrada que tengan body.
    Por ejemplo, peticiones POST, PUT o DELETE
    """
    try:
        return json.loads(event["body"]), None
    except Exception as ex:
        return None, ex


def get_query(query_str: str) -> dict:
    """
    Transforma el query string de entrada en un diccionario con filtros para
    realizar la consulta a la base de datos
    Ejemplo:
        query=espacio_academico_id:1,proyecto_curricular_id:375, ...
    """
    query_total = {}
    int_fields = ['version',
                  'espacio_academico_id',
                  'proyecto_curricular_id',
                  'plan_estudios_id',
                  'flujo.asignado_a',
                  'flujo.asignado_por'
                  ]
    key_map = {
        "proyecto_curricular_id": "proyecto_curricular_ids",
        "plan_estudios_id": "plan_estudios_ids"
    }
    keys_map = list(key_map.keys())
    for cond in query_str.split(","):
        kv = cond.split(":", 1)
        if len(kv) == 2:
            k, v = kv
            if v == 'false':
                v = False
            elif v == 'true':
                v = True

            # estado:asignado|rechazado|expirado => {"$in": [...]}
            if k == 'estado' and isinstance(v, str) and '|' in v:
                query_total[k] = {"$in": v.split('|')}
                continue

            if k in int_fields:
                v = int(v)
            elif k == 'syllabus_code':
                v = uuid.UUID(v)

            if k == "_id":
                v = ObjectId(v)
        else:
            k, v = kv[0], None

        if k in keys_map:
            k = key_map[k]
            query_total[k] = v
        else:
            query_total[k] = v
    return query_total


def get_sort_by(query_params) -> list:
    """
    Transforma el query params sortby en la lista de consulta sort
    para la base de datos combinando el sortby con el order (ASCENDENTE,
    DESCENDENTE)
    Ejemplo:
        sortby=espacio_academico_id, ...
        order=asc, ...
    """
    sort_by_total = []
    if query_params.get("sortby"):
        sort_by_list = str(query_params.get("sortby")).split(",")
        if query_params.get("order"):
            order_list = str(query_params.get("order")).split(",")
            if len(order_list) == 1:
                # Default ASCENDING
                order_label = ORDER_LABEL.get(query_params.get("order"), ASCENDING)
                sort_by_total = [(e, order_label) for e in sort_by_list]
            elif len(order_list) == len(sort_by_list):
                for i, e in enumerate(sort_by_list):
                    order_label = ORDER_LABEL.get(order_list[i], ASCENDING)
                    sort_by_total.append((e, order_label))
            else:
                # Default ASCENDING
                sort_by_total = [(e, ASCENDING) for e in sort_by_list]
    return sort_by_total


def parse_query_params(event) -> tuple:
    """
    Organiza los query params query, fields, sortby, order, limit, offset generando
    la estructura para realizar la consulta a base de datos
    """
    try:
        print("event: ", event)
        query_params_result = {
            "limit": 10
        }
        query_params = event["queryStringParameters"]
        if isinstance(query_params, dict):
            # query: k:v, k: v
            if query_params.get("query"):
                query_params_result["filter"] = get_query(str(query_params.get("query")))

            # fields: col1, col2, entity.col3
            if query_params.get("fields"):
                query_params_result["projection"] = str(query_params.get("fields")).split(",")

            # sortby: col1,col2
            # order: desc,asc
            if query_params.get("sortby"):
                query_params_result["sort"] = get_sort_by(query_params)

            # limit: 10 (default is 10)
            if query_params.get("limit"):
                query_params_result["limit"] = int(query_params.get("limit"))

            # offset: 0 (default is 0)
            if query_params.get("offset"):
                query_params_result["skip"] = int(query_params.get("offset"))

            return query_params_result, None
        else:
            return query_params_result, None
    except Exception as ex:
        print("Error in parse_query_params")
        print(ex)
        return {}, ex


def set_version(syllabus_data: dict, syllabus_collection):
    """
    Calcula el versionado de los syllabus por syllabus_code.
    Los borradores cancelados no cuentan para evitar huecos en la numeración.
    """
    try:
        if syllabus_data.get("syllabus_code"):
            syllabus_code = str(syllabus_data.get("syllabus_code"))
            query = {
                "syllabus_code": uuid.UUID(syllabus_code),
                "estado": {"$ne": ESTADO_CANCELADO}
            }
            total = syllabus_collection.count_documents(query)
            syllabus_data["version"] = total + 1
        else:
            syllabus_data["version"] = 1
    except Exception as ex:
        print(f"Error assigning version. Details:  {str(ex)}")


def update_old_syllabus(syllabus_data: dict, syllabus_collection):
    """
    Marca todas las versiones anteriores del mismo syllabus como no actuales e inactivas.
    Esto garantiza que solo una versión esté activa por syllabus_code.
    """
    try:
        if syllabus_data.get("syllabus_code"):
            syllabus_code = str(syllabus_data.get("syllabus_code"))
            query = {
                "syllabus_code": uuid.UUID(syllabus_code)
            }
            syllabus_collection.update_many(
                query,
                {
                    "$set": {"syllabus_actual": False}
                    # "$set": {"syllabus_actual": False, "activo": False}
                }
            )
    except Exception as ex:
        print(f"Error updating old syllabus. Details: {str(ex)}")


def existe_borrador_abierto(syllabus_code, syllabus_collection) -> bool:
    """Indica si el syllabus_code ya tiene una propuesta en curso sin resolver."""
    try:
        query = {
            "syllabus_code": syllabus_code,
            "estado": {"$in": list(ESTADOS_BORRADOR)},
            "activo": True
        }
        return syllabus_collection.count_documents(query) > 0
    except Exception as ex:
        print(f"Error checking open draft. Details: {str(ex)}")
        return False


def validate_syllabus_data_integrity(syllabus_data: dict):
    """
    Valida que los IDs de resultados de aprendizaje referenciados en 
    evaluación realmente existan en la sección de resultados_aprendizaje.
    
    Ajustado para manejar tu estructura actual de datos.
    """
    try:
        if not syllabus_data.get("evaluacion") or not syllabus_data.get("resultados_aprendizaje"):
            print("Validación de integridad: No hay evaluación o resultados para validar")
            return
        
        # Extrae todos los IDs de resultados disponibles
        available_ids = set()
        resultados_aprendizaje = syllabus_data["resultados_aprendizaje"]
        
        for competencia in resultados_aprendizaje:
            # Maneja tanto objetos Pydantic como diccionarios
            if hasattr(competencia, 'resultados'):
                resultados = competencia.resultados
            else:
                resultados = competencia.get("resultados", [])
            
            for resultado in resultados:
                if hasattr(resultado, 'id'):
                    available_ids.add(resultado.id)
                else:
                    available_ids.add(resultado.get("id"))
        
        print(f"IDs de resultados disponibles: {available_ids}")
        
        # Verifica IDs referenciados en cada tipo de evaluación
        evaluacion_data = syllabus_data["evaluacion"]
        
        if hasattr(evaluacion_data, 'tipos_evaluacion'):
            tipos_evaluacion = evaluacion_data.tipos_evaluacion
        elif isinstance(evaluacion_data, dict) and "tipos_evaluacion" in evaluacion_data:
            tipos_evaluacion = evaluacion_data["tipos_evaluacion"]
        else:
            tipos_evaluacion = []
        
        for tipo_eval in tipos_evaluacion:
            if hasattr(tipo_eval, 'nombre'):
                nombre_eval = tipo_eval.nombre
                ids_asociados = tipo_eval.resultados_aprendizaje_asociados
            else:
                nombre_eval = tipo_eval.get("nombre", "Sin nombre")
                ids_asociados = tipo_eval.get("resultados_aprendizaje_asociados", [])
            
            for id_resultado in ids_asociados:
                if id_resultado and id_resultado not in available_ids:
                    print(f"Advertencia: ID de resultado '{id_resultado}' en evaluación '{nombre_eval}' no existe en resultados_aprendizaje")
                        
    except Exception as ex:
        print(f"Error en validación de consistencia: {ex}")


# Formato de respuestas
def format_specific_values(result):
    """
    Convierte valores con tipo de dato específico a un formato que
    pueda ser respondido como JSON.
    Ejemplo:
        ObjectId -> str
        datetime -> str
        UUID -> str
    """
    if result.get("_id"):
        result["_id"] = str(result["_id"])
    if result.get("fecha_creacion"):
        result["fecha_creacion"] = str(result["fecha_creacion"])
    if result.get("fecha_modificacion"):
        result["fecha_modificacion"] = str(result["fecha_modificacion"])
    if result.get("syllabus_code"):
        result["syllabus_code"] = str(result["syllabus_code"])
    return result


def format_response(result, message: str, status_code: int, success: bool):
    """
    Crea la estructura de respuesta con los campos:
        statusCode: código HTTP
        body: JSON con la siguiente estructura
            Success: Campo booleano que indica si fue exitosa (true) o no la petición
            Status: código HTTP
            Message: mensaje descriptivo del resultado de la petición
            Data: No es agregado en caso de un error en las peticiones, contiene una lista
                o diccionario con el resultado de la petición entrante.
    """
    if isinstance(result, dict):
        if success:
            result = format_specific_values(result)

            return {"statusCode": status_code,
                    "body": json.dumps({
                        "Success": success,
                        "Status": status_code,
                        "Message": message,
                        "Data": result
                    })}
        else:
            return {"statusCode": status_code,
                    "body": json.dumps({
                        "Success": success,
                        "Status": status_code,
                        "Message": message
                    })}
    elif isinstance(result, list):
        result_total = []
        if success:
            for res in result:
                result_total.append(format_specific_values(res))
            return {"statusCode": status_code,
                    "body": json.dumps({
                        "Success": success,
                        "Status": status_code,
                        "Message": message,
                        "Data": result_total
                    })}
        else:
            return {"statusCode": status_code,
                    "body": json.dumps({
                        "Success": success,
                        "Status": status_code,
                        "Message": message
                    })}
    elif result is None:
        if success:
            return {"statusCode": status_code,
                    "body": json.dumps({
                        "Success": success,
                        "Status": status_code,
                        "Message": message
                    })}
        else:
            return {"statusCode": status_code,
                    "body": json.dumps({
                        "Success": success,
                        "Status": status_code,
                        "Message": message
                    })}


def create_syllabus(syllabus_data, syllabus_collection):
    try:
        if syllabus_data.get("syllabus_code"):
            syllabus_data["syllabus_code"] = uuid.UUID(syllabus_data.get("syllabus_code"))
        else:
            syllabus_data["syllabus_code"] = uuid.uuid4()

        estado = estado_documento(syllabus_data)
        es_borrador = estado in ESTADOS_BORRADOR

        # No permitir más de una propuesta abierta por syllabus_code
        if es_borrador and existe_borrador_abierto(syllabus_data["syllabus_code"], syllabus_collection):
            return format_response(
                {},
                "Ya existe una propuesta en curso para este syllabus",
                409,
                False)

        set_version(syllabus_data, syllabus_collection)

        if es_borrador:
            # Un borrador NO despublica la versión vigente ni queda como versión actual
            syllabus_data["syllabus_actual"] = False
            syllabus_data["estado"] = estado
            flujo = syllabus_data.get("flujo") or {}
            if not flujo.get("fecha_asignacion"):
                flujo["fecha_asignacion"] = local_now().isoformat()
            flujo.setdefault("historial", [])
            flujo["historial"].append(build_historial_entry(
                estado,
                tercero_id=flujo.get("asignado_por"),
                tercero_documento=flujo.get("asignado_por_documento")))
            syllabus_data["flujo"] = flujo
        else:
            update_old_syllabus(syllabus_data, syllabus_collection)
            syllabus_data["syllabus_actual"] = True
            syllabus_data["estado"] = ESTADO_ACTIVO

        # Validación de integridad para el modelo actual
        validate_syllabus_data_integrity(syllabus_data)
        
        print("Inserting new syllabus")
        result = syllabus_collection.insert_one(syllabus_data)
        print("Created new syllabus")
        if result:
            new_syllabus_id = result.inserted_id
            new_syllabus = syllabus_collection.find_one(new_syllabus_id)
            return format_response(
                new_syllabus,
                "Created syllabus",
                201,
                True)
        else:
            return format_response(
                {},
                "Syllabus was not created",
                400,
                False)
    except errors.NetworkTimeout as nex:
        print("Error querying all syllabus")
        print(f"Detail: {nex}")
        return format_response(
            {},
            "Error registering new syllabus! Detail: Network Timeout",
            500,
            False)
    except errors.ServerSelectionTimeoutError as sst_err:
        print("Error querying all syllabus")
        print(f"Detail: {sst_err}")
        return format_response(
            {},
            "Error registering new syllabus! Detail: Server Selection Timeout Error",
            500,
            False)
    except Exception as ex:
        print("Error creating register syllabus")
        print(f"Detail: {ex}")
        return format_response(
            {},
            "Error registering new syllabus! Detail: general error, for more details check the logs",
            500,
            False)


def prepare_update_payload(data: dict) -> dict:
    """
    Valida y construye el $set de una actualización de contenido.

    La actualización es parcial: solo se actualizan los campos enviados
    explícitamente (no se rellenan con valores por defecto del modelo, para no
    sobrescribir datos con None). ``syllabus_code`` es inmutable: solo se
    convierte a UUID si viene, nunca se persiste (update_syllabus lo descarta).
    """
    SyllabusUpdateModel(**data)  # valida la estructura (campos opcionales)
    payload = dict(data)
    code = payload.get("syllabus_code")
    if code:
        payload["syllabus_code"] = uuid.UUID(str(code))
    payload["fecha_modificacion"] = local_now()
    return payload


def update_syllabus(syllabus_id, syllabus_data, syllabus_collection):
    try:
        filter_ = {"_id": ObjectId(syllabus_id)}

        current = syllabus_collection.find_one(filter_)
        if not current:
            return format_response(
                {},
                "Syllabus not found",
                404,
                False)

        # El contenido no puede modificarse cuando la propuesta está en revisión o cancelada
        origen = estado_documento(current)
        if origen in ESTADOS_NO_EDITABLES_CONTENIDO:
            return format_response(
                {},
                f"No se puede editar el contenido en estado '{origen}'",
                409,
                False)

        # Validación de integridad antes de actualizar
        validate_syllabus_data_integrity(syllabus_data)

        # Los campos de flujo/versionado no se modifican con una edición de contenido
        for campo in CAMPOS_PROTEGIDOS_UPDATE:
            syllabus_data.pop(campo, None)

        print("Updating syllabus")
        result = syllabus_collection.update_one(
            filter_,
            {"$set": syllabus_data})
        if result.modified_count:
            print(f"Updated syllabus {syllabus_id}")
            syllabus = syllabus_collection.find_one(filter_)
            return format_response(
                syllabus,
                "Updated syllabus",
                200,
                True)
        else:
            return format_response(
                {},
                "Syllabus not updated",
                400,
                False)
    except errors.NetworkTimeout as nex:
        print("Error querying all syllabus")
        print(f"Detail: {nex}")
        return format_response(
            {},
            "Error updating syllabus! Detail: Network Timeout",
            500,
            False)
    except errors.ServerSelectionTimeoutError as sst_err:
        print("Error querying all syllabus")
        print(f"Detail: {sst_err}")
        return format_response(
            {},
            "Error updating syllabus! Detail: Server Selection Timeout Error",
            500,
            False)
    except Exception as ex:
        print("Error updating syllabus")
        print(f"Detail: {ex}")
        return format_response(
            {},
            "Error updating syllabus! Detail: general error, for more details check the logs",
            500,
            False)


def update_syllabus_estado(syllabus_id, payload: dict, syllabus_collection):
    """
    Aplica una transición de estado validada por la máquina de estados y
    registra la auditoría correspondiente en flujo.historial.
    """
    try:
        filter_ = {"_id": ObjectId(syllabus_id)}
        current = syllabus_collection.find_one(filter_)
        if not current:
            return format_response({}, "Syllabus not found", 404, False)

        origen = estado_documento(current)
        destino = payload.get("estado")

        if not destino:
            return format_response({}, "El campo 'estado' es obligatorio", 400, False)

        if destino in (ESTADO_ACTIVO, ESTADO_APROBADO):
            return format_response(
                {},
                "La publicación/aprobación debe realizarse mediante el endpoint /publicar",
                409,
                False)

        if not validar_transicion(origen, destino):
            return format_response(
                {},
                f"Transición no permitida: {origen} -> {destino}",
                409,
                False)

        now_iso = local_now().isoformat()
        flujo = current.get("flujo") or {}

        # Reasignación de docente
        if payload.get("asignado_a") is not None:
            flujo["asignado_a"] = payload.get("asignado_a")
            flujo["asignado_a_documento"] = payload.get("asignado_a_documento")
            flujo["fecha_asignacion"] = now_iso

        # Plazo del docente
        if payload.get("fecha_limite"):
            flujo["fecha_limite"] = payload.get("fecha_limite")

        if destino == ESTADO_EN_REVISION:
            flujo["fecha_envio"] = now_iso
        elif destino == ESTADO_RECHAZADO:
            flujo["motivo_rechazo"] = payload.get("motivo_rechazo")
            flujo["fecha_ultima_decision"] = now_iso
            flujo["ultima_decision_por"] = payload.get("tercero_id")
        elif destino == ESTADO_CANCELADO:
            flujo["fecha_ultima_decision"] = now_iso
            flujo["ultima_decision_por"] = payload.get("tercero_id")
        elif destino == ESTADO_ASIGNADO and origen in (ESTADO_RECHAZADO, ESTADO_EXPIRADO):
            # Reapertura / renovación de plazo tras un rechazo o vencimiento
            flujo["motivo_rechazo"] = None
            flujo["fecha_ultima_decision"] = None
            flujo["fecha_asignacion"] = now_iso

        flujo.setdefault("historial", [])
        flujo["historial"].append(build_historial_entry(
            destino,
            tercero_id=payload.get("tercero_id"),
            tercero_documento=payload.get("tercero_documento"),
            observacion=payload.get("observacion") or payload.get("motivo_rechazo")))

        update_fields = {
            "estado": destino,
            "flujo": flujo,
            "fecha_modificacion": local_now()
        }
        if destino == ESTADO_CANCELADO:
            update_fields["activo"] = False

        syllabus_collection.update_one(filter_, {"$set": update_fields})
        syllabus = syllabus_collection.find_one(filter_)
        print(f"Transitioned syllabus {syllabus_id}: {origen} -> {destino}")
        return format_response(syllabus, "Estado actualizado", 200, True)
    except errors.NetworkTimeout as nex:
        print("Error updating syllabus state")
        print(f"Detail: {nex}")
        return format_response(
            {},
            "Error updating syllabus state! Detail: Network Timeout",
            500,
            False)
    except errors.ServerSelectionTimeoutError as sst_err:
        print("Error updating syllabus state")
        print(f"Detail: {sst_err}")
        return format_response(
            {},
            "Error updating syllabus state! Detail: Server Selection Timeout Error",
            500,
            False)
    except Exception as ex:
        print("Error updating syllabus state")
        print(f"Detail: {ex}")
        return format_response(
            {},
            "Error updating syllabus state! Detail: general error, for more details check the logs",
            500,
            False)


def publicar_syllabus(syllabus_id, payload: dict, syllabus_collection):
    """
    Publica (aprueba) una propuesta: la marca como versión actual y despublica
    las demás versiones del mismo syllabus_code.
    """
    try:
        filter_ = {"_id": ObjectId(syllabus_id)}
        current = syllabus_collection.find_one(filter_)
        if not current:
            return format_response({}, "Syllabus not found", 404, False)

        origen = estado_documento(current)
        if origen != ESTADO_EN_REVISION:
            return format_response(
                {},
                f"Solo una propuesta en revisión puede publicarse (estado actual: {origen})",
                409,
                False)

        syllabus_code = current.get("syllabus_code")
        now = local_now()
        now_iso = now.isoformat()

        # 1) Despublicar las demás versiones del mismo código
        if syllabus_code:
            syllabus_collection.update_many(
                {"syllabus_code": syllabus_code, "_id": {"$ne": current["_id"]}},
                {"$set": {"syllabus_actual": False}})

        flujo = current.get("flujo") or {}
        flujo["fecha_ultima_decision"] = now_iso
        flujo["ultima_decision_por"] = payload.get("tercero_id")
        flujo.setdefault("historial", [])
        flujo["historial"].append(build_historial_entry(
            ESTADO_ACTIVO,
            tercero_id=payload.get("tercero_id"),
            tercero_documento=payload.get("tercero_documento"),
            observacion=payload.get("observacion") or "Propuesta aprobada"))

        update_fields = {
            "syllabus_actual": True,
            "estado": ESTADO_ACTIVO,
            "flujo": flujo,
            "fecha_modificacion": now
        }
        # Al publicar se registran el revisor/aprobador (coordinador que aprueba),
        # la fecha de aprobación y el acta de consejo.
        seguimiento = dict(current.get("seguimiento") or {})
        nombre_aprobador = payload.get("nombre_aprobador")
        if nombre_aprobador:
            seguimiento["reviso"] = nombre_aprobador
            seguimiento["aprobo"] = nombre_aprobador
        seguimiento["fechaAprobacionConsejo"] = now_iso
        if payload.get("numeroActa"):
            seguimiento["numeroActa"] = payload.get("numeroActa")
        if payload.get("archivo"):
            seguimiento["archivo"] = payload.get("archivo")
        update_fields["seguimiento"] = seguimiento

        # 2) Publicar la propuesta aprobada
        syllabus_collection.update_one(filter_, {"$set": update_fields})
        syllabus = syllabus_collection.find_one(filter_)
        print(f"Published syllabus {syllabus_id}")
        return format_response(syllabus, "Propuesta aprobada y publicada", 200, True)
    except errors.NetworkTimeout as nex:
        print("Error publishing syllabus")
        print(f"Detail: {nex}")
        return format_response(
            {},
            "Error publishing syllabus! Detail: Network Timeout",
            500,
            False)
    except errors.ServerSelectionTimeoutError as sst_err:
        print("Error publishing syllabus")
        print(f"Detail: {sst_err}")
        return format_response(
            {},
            "Error publishing syllabus! Detail: Server Selection Timeout Error",
            500,
            False)
    except Exception as ex:
        print("Error publishing syllabus")
        print(f"Detail: {ex}")
        return format_response(
            {},
            "Error publishing syllabus! Detail: general error, for more details check the logs",
            500,
            False)


def delete_syllabus(syllabus_id, syllabus_data, syllabus_collection):
    try:
        filter_ = {"_id": ObjectId(syllabus_id)}
        
        # Excluir el campo _id de la actualización ya que es inmutable en MongoDB
        if "_id" in syllabus_data:
            del syllabus_data["_id"]
            
        print("Deleting syllabus")
        result = syllabus_collection.update_one(
            filter_,
            {"$set": syllabus_data})
        print(f"Deleted syllabus {syllabus_id}")
        if result.modified_count:
            return format_response(
                None,
                "Deleted syllabus",
                204,
                True
            )
        else:
            return format_response(
                None,
                "Syllabus not deleted",
                400,
                False)
    except errors.NetworkTimeout as nex:
        print("Error querying all syllabus")
        print(f"Detail: {nex}")
        return format_response(
            {},
            "Error deleting syllabus! Detail: Network Timeout",
            500,
            False)
    except errors.ServerSelectionTimeoutError as sst_err:
        print("Error querying all syllabus")
        print(f"Detail: {sst_err}")
        return format_response(
            {},
            "Error deleting syllabus! Detail: Server Selection Timeout Error",
            500,
            False)
    except Exception as ex:
        print("Error deleting syllabus")
        print(f"Detail: {ex}")
        return format_response(
            {},
            "Error deleting syllabus! Detail: general error, for more details check the logs",
            500,
            False)


def get_all_syllabus(query_complement, syllabus_collection):
    try:
        print("Query")
        print(query_complement)
        # Por defecto los borradores/propuestas no son visibles; solo si el
        # llamador filtra explícitamente por estado se incluyen.
        filt = query_complement.get("filter") or {}
        if "estado" not in filt:
            query_complement["filter"] = {**filt, "estado": {"$nin": list(ESTADOS_BORRADOR)}}
        syllabus = list(syllabus_collection.find(**query_complement))
        print(f"Consulted record.")
        if syllabus:
            print("Syllabus found!")
            return format_response(
                syllabus,
                "Request successful",
                200,
                True)
        else:
            print("Syllabus not found!")
            return format_response(
                [],
                "Request successful",
                200,
                True)
    except errors.NetworkTimeout as nex:
        print("Error querying all syllabus")
        print(f"Detail: {nex}")
        return format_response(
            {},
            "Error querying all syllabus! Detail: Network Timeout",
            500,
            False)
    except errors.ServerSelectionTimeoutError as sst_err:
        print("Error querying all syllabus")
        print(f"Detail: {sst_err}")
        return format_response(
            {},
            "Error querying all syllabus! Detail: Server Selection Timeout Error",
            500,
            False)
    except Exception as ex:
        print("Error querying all syllabus")
        print(f"Detail: {ex}")
        return format_response(
            {},
            "Error querying all syllabus! Detail: general error, for more details check the logs",
            500,
            False)


def get_one_syllabus(syllabus_code, syllabus_collection):
    try:
        syllabus = syllabus_collection.find_one({
            "syllabus_code": uuid.UUID(syllabus_code),
            "syllabus_actual": True
        })
        print(f"Consulted record.")
        if syllabus:
            print("Syllabus found!")
            print(syllabus)
            print(type(syllabus))
            return format_response(
                syllabus,
                "Syllabus OK",
                200,
                True)
        else:
            print("Syllabus not found!")
            return format_response(
                {},
                "Syllabus Not Found",
                404,
                False)
    except Exception as ex:
        print("Error querying a syllabus")
        print(f"Detail: {ex}")
        return format_response(
            {},
            "Error querying a syllabus! Detail: general error, for more details check the logs",
            500,
            False)


def lambda_handler(event, context):
    client = None
    try:
        http_method = event['httpMethod']
        if http_method == 'POST':
            data, error = parse_body(event)
            if error is None:
                try:
                    # Valida la estructura usando el modelo flexible
                    syllabus_model = SyllabusCreationModel(**data)
                    print("Validación del modelo de creación exitosa")
                    
                    # Convierte completamente a diccionarios para MongoDB
                    syllabus_data = syllabus_model.dict()
                    print("Conversión a diccionarios para MongoDB exitosa")
                    
                except Exception as validation_error:
                    print(f"Error de validación en modelo de creación: {validation_error}")
                    return format_response(
                        {},
                        f"Error en estructura de datos: {validation_error}",
                        400,
                        False)
                
                client = connect_db_client()
                if client:
                    syllabus_collection = client[str(SYLLABUS_CRUD_DB)]["syllabus"]
                    response = create_syllabus(syllabus_data, syllabus_collection)
                    close_connect_db(client)
                    return response

                return format_response(
                    {},
                    "Error registering new syllabus!",
                    500,
                    False)
            else:
                print(error)
                return format_response(
                    {},
                    "Error registering new syllabus! Detail: Error in input data",
                    500,
                    False)
        elif http_method == 'PUT':
            data, error = parse_body(event)
            if error is None:
                resource_path = str(event.get("resource") or event.get("path") or "").rstrip("/")

                if resource_path.endswith("/estado"):
                    try:
                        model = SyllabusTransitionModel(**data)
                    except Exception as validation_error:
                        print(f"Error de validación en transición de estado: {validation_error}")
                        return format_response(
                            {},
                            f"Error en estructura de datos: {validation_error}",
                            400,
                            False)
                    client = connect_db_client()
                    if client:
                        syllabus_collection = client[str(SYLLABUS_CRUD_DB)]["syllabus"]
                        response = update_syllabus_estado(
                            event["pathParameters"]["id"], model.dict(), syllabus_collection)
                        close_connect_db(client)
                        return response
                    return format_response(
                        {},
                        "Error updating syllabus state!",
                        500,
                        False)

                if resource_path.endswith("/publicar"):
                    try:
                        model = SyllabusPublishModel(**data)
                    except Exception as validation_error:
                        print(f"Error de validación en publicación: {validation_error}")
                        return format_response(
                            {},
                            f"Error en estructura de datos: {validation_error}",
                            400,
                            False)
                    client = connect_db_client()
                    if client:
                        syllabus_collection = client[str(SYLLABUS_CRUD_DB)]["syllabus"]
                        response = publicar_syllabus(
                            event["pathParameters"]["id"], model.dict(), syllabus_collection)
                        close_connect_db(client)
                        return response
                    return format_response(
                        {},
                        "Error publishing syllabus!",
                        500,
                        False)

                try:
                    syllabus_id = event["pathParameters"]["id"]

                    # Validación y construcción del $set parcial (tolerante:
                    # no exige campos inmutables ni no enviados)
                    syllabus_data = prepare_update_payload(data)
                    print("Validación del modelo de actualización exitosa")

                except Exception as validation_error:
                    print(f"Error de validación en modelo de actualización: {validation_error}")
                    return format_response(
                        {},
                        f"Error en estructura de datos: {validation_error}",
                        400,
                        False)
                
                client = connect_db_client()
                if client:
                    syllabus_collection = client[str(SYLLABUS_CRUD_DB)]["syllabus"]
                    response = update_syllabus(syllabus_id, syllabus_data, syllabus_collection)
                    close_connect_db(client)
                    return response

                return format_response(
                    {},
                    "Error updating syllabus!",
                    500,
                    False)
            else:
                print(error)
                return format_response(
                    {},
                    "Error updating syllabus! Detail: Error in input data",
                    500,
                    False)
        elif http_method == 'DELETE':
            syllabus_id = event["pathParameters"]["id"]
            print(syllabus_id)
            # Create structure to update (delete logic)
            syllabus_data = DeleteSyllabusModel().__dict__
            client = connect_db_client()
            if client:
                syllabus_collection = client[str(SYLLABUS_CRUD_DB)]["syllabus"]
                response = delete_syllabus(syllabus_id, syllabus_data, syllabus_collection)
                close_connect_db(client)
                return response

            return format_response(
                None,
                "Error deleting syllabus!",
                500,
                False)
        elif http_method == 'GET':
            client = connect_db_client()
            if client:
                syllabus_collection = client[str(SYLLABUS_CRUD_DB)]["syllabus"]
                if 'pathParameters' in event and event['pathParameters'] is not None:
                    syllabus_code = event["pathParameters"]["id"]
                    print(syllabus_code)
                    response = get_one_syllabus(syllabus_code, syllabus_collection)
                    close_connect_db(client)
                    return response
                else:
                    query_complement, err = parse_query_params(event)
                    if err is None:
                        response = get_all_syllabus(query_complement, syllabus_collection)
                        close_connect_db(client)
                        return response
                    else:
                        return format_response(
                            {},
                            "Error service GetAll: The request contains an incorrect parameter or no record exists",
                            404,
                            True)
            return format_response(
                {},
                "Error get syllabus!",
                500,
                False)
        else:
            close_connect_db(client)
            return format_response(
                {},
                f"HTTP method not allowed",
                500,
                False)
    except Exception as ex:
        print("Error in syllabus request")
        print(f"Detail: {ex}")
        close_connect_db(client)
        return format_response(
            {},
            f"Error in syllabus request! Detail: {ex}",
            500,
            False)
