"""Pruebas de la máquina de estados del flujo de aprobación del CRUD de syllabus.

Se usa una colección Mongo falsa en memoria para probar las transiciones y las
reglas de visibilidad sin depender de una base de datos real.
"""

import json
import os
import sys
import uuid

import pytest
from bson import ObjectId

# El módulo lee TIMEZONE al importarse; se fija antes de importar.
os.environ.setdefault("TIMEZONE", "America/Bogota")
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from src.handlers.crud_syllabus import app  # noqa: E402


class _Result:
    def __init__(self, inserted_id=None, modified_count=0):
        self.inserted_id = inserted_id
        self.modified_count = modified_count


class _FakeCollection:
    """Colección Mongo mínima en memoria con los operadores usados por el CRUD."""

    def __init__(self):
        self.docs = []
        self._seq = 0

    @staticmethod
    def _match_value(doc_value, cond):
        if isinstance(cond, dict):
            if "$in" in cond:
                return doc_value in cond["$in"]
            if "$nin" in cond:
                return doc_value not in cond["$nin"]
            if "$ne" in cond:
                return doc_value != cond["$ne"]
        return doc_value == cond

    def _match(self, doc, query):
        for key, cond in query.items():
            if not self._match_value(doc.get(key), cond):
                return False
        return True

    def count_documents(self, query):
        return sum(1 for d in self.docs if self._match(d, query))

    def insert_one(self, doc):
        self._seq += 1
        stored = dict(doc)
        stored.setdefault("_id", ObjectId())
        self.docs.append(stored)
        return _Result(inserted_id=stored["_id"])

    def find_one(self, query):
        if not isinstance(query, dict):
            query = {"_id": query}
        for d in self.docs:
            if self._match(d, query):
                return dict(d)
        return None

    def find(self, **kwargs):
        filt = kwargs.get("filter") or {}
        limit = kwargs.get("limit")
        result = [dict(d) for d in self.docs if self._match(d, filt)]
        if limit:
            result = result[:limit]
        return result

    def update_one(self, query, update):
        for d in self.docs:
            if self._match(d, query):
                self._apply(d, update["$set"])
                return _Result(modified_count=1)
        return _Result(modified_count=0)

    def update_many(self, query, update):
        count = 0
        for d in self.docs:
            if self._match(d, query):
                self._apply(d, update["$set"])
                count += 1
        return _Result(modified_count=count)

    @staticmethod
    def _apply(doc, sets):
        for key, value in sets.items():
            if "." in key:
                parts = key.split(".")
                cursor = doc
                for part in parts[:-1]:
                    cursor = cursor.setdefault(part, {})
                cursor[parts[-1]] = value
            else:
                doc[key] = value


def _body(response):
    return json.loads(response["body"])


def _crear_borrador(collection, syllabus_code, docente=99, coordinador=88):
    data = app.SyllabusCreationModel(
        syllabus_code=str(syllabus_code),
        estado=app.ESTADO_ASIGNADO,
        espacio_academico_id=1,
        proyecto_curricular_ids=[10],
        plan_estudios_ids=[20],
        flujo={
            "asignado_a": docente,
            "asignado_a_documento": "80111222",
            "asignado_por": coordinador,
            "asignado_por_documento": "79333444",
            "fecha_limite": "2026-12-31",
        },
    ).dict()
    return app.create_syllabus(data, collection)


def _crear_activo(collection, syllabus_code):
    data = app.SyllabusCreationModel(
        syllabus_code=str(syllabus_code),
        estado=app.ESTADO_ACTIVO,
        espacio_academico_id=1,
        proyecto_curricular_ids=[10],
        plan_estudios_ids=[20],
    ).dict()
    return app.create_syllabus(data, collection)


def test_validar_transicion():
    assert app.validar_transicion(app.ESTADO_ASIGNADO, app.ESTADO_EN_REVISION)
    assert app.validar_transicion(app.ESTADO_EN_REVISION, app.ESTADO_RECHAZADO)
    assert app.validar_transicion(app.ESTADO_RECHAZADO, app.ESTADO_ASIGNADO)
    assert app.validar_transicion(app.ESTADO_EXPIRADO, app.ESTADO_ASIGNADO)
    assert not app.validar_transicion(app.ESTADO_ACTIVO, app.ESTADO_EN_REVISION)
    assert not app.validar_transicion(app.ESTADO_CANCELADO, app.ESTADO_ASIGNADO)


def test_estado_documento_retrocompatible():
    assert app.estado_documento({}) == app.ESTADO_ACTIVO
    assert app.estado_documento({"estado": app.ESTADO_ASIGNADO}) == app.ESTADO_ASIGNADO


def test_get_query_estado_multiple():
    query = app.get_query("estado:asignado|rechazado|expirado")
    assert query["estado"] == {"$in": ["asignado", "rechazado", "expirado"]}


def test_get_query_flujo_asignado_entero():
    query = app.get_query("flujo.asignado_a:1042")
    assert query["flujo.asignado_a"] == 1042


def test_crear_borrador_no_despublica():
    collection = _FakeCollection()
    code = uuid.uuid4()
    _crear_activo(collection, code)

    response = _crear_borrador(collection, code)
    draft = _body(response)["Data"]

    assert draft["estado"] == app.ESTADO_ASIGNADO
    assert draft["syllabus_actual"] is False
    # La versión activa sigue vigente
    activo = [d for d in collection.docs if d["syllabus_actual"] is True]
    assert len(activo) == 1
    assert activo[0]["estado"] == app.ESTADO_ACTIVO


def test_no_segundo_borrador_abierto():
    collection = _FakeCollection()
    code = uuid.uuid4()
    _crear_activo(collection, code)
    _crear_borrador(collection, code)

    response = _crear_borrador(collection, code)
    assert response["statusCode"] == 409
    assert _body(response)["Success"] is False


def test_ciclo_completo_aprobacion():
    collection = _FakeCollection()
    code = uuid.uuid4()
    _crear_activo(collection, code)
    draft = _body(_crear_borrador(collection, code))["Data"]
    draft_id = draft["_id"]

    # El docente propone
    assert _body(app.update_syllabus_estado(
        draft_id, {"estado": app.ESTADO_EN_REVISION, "tercero_id": 99}, collection))["Data"]["estado"] == app.ESTADO_EN_REVISION

    # El contenido queda bloqueado en revisión
    blocked = _body(app.update_syllabus(draft_id, {"justificacion": "x"}, collection))
    assert blocked["Success"] is False

    # El coordinador rechaza y reabre con nuevo plazo
    rechazado = _body(app.update_syllabus_estado(
        draft_id,
        {"estado": app.ESTADO_RECHAZADO, "motivo_rechazo": "Faltan datos", "tercero_id": 88},
        collection))["Data"]
    assert rechazado["estado"] == app.ESTADO_RECHAZADO
    assert rechazado["flujo"]["fecha_ultima_decision"]
    assert rechazado["flujo"]["ultima_decision_por"] == 88
    reopened = _body(app.update_syllabus_estado(
        draft_id,
        {"estado": app.ESTADO_ASIGNADO, "fecha_limite": "2027-01-15", "tercero_id": 88},
        collection))["Data"]
    assert reopened["estado"] == app.ESTADO_ASIGNADO
    assert reopened["flujo"]["fecha_limite"] == "2027-01-15"
    # Al reabrir se limpia el snapshot de la última decisión
    assert reopened["flujo"]["fecha_ultima_decision"] is None

    # Propone de nuevo y se publica
    app.update_syllabus_estado(draft_id, {"estado": app.ESTADO_EN_REVISION, "tercero_id": 99}, collection)
    published = _body(app.publicar_syllabus(
        draft_id, {"tercero_id": 88, "nombre_aprobador": "Coordinador X"}, collection))["Data"]

    assert published["estado"] == app.ESTADO_ACTIVO
    assert published["syllabus_actual"] is True
    assert published["seguimiento"]["aprobo"] == "Coordinador X"
    assert published["flujo"]["fecha_ultima_decision"]
    assert published["flujo"]["ultima_decision_por"] == 88

    # Solo una versión activa por código
    activos = [d for d in collection.docs if d["syllabus_actual"] is True]
    assert len(activos) == 1
    assert str(activos[0]["_id"]) == draft_id

    # Auditoría registrada
    estados_historial = [h["estado"] for h in published["flujo"]["historial"]]
    assert app.ESTADO_ACTIVO in estados_historial


def test_visibilidad_excluye_borradores_por_defecto():
    collection = _FakeCollection()
    _crear_activo(collection, uuid.uuid4())
    _crear_borrador(collection, uuid.uuid4())

    response = _body(app.get_all_syllabus({}, collection))
    estados = {d["estado"] for d in response["Data"]}
    assert estados == {app.ESTADO_ACTIVO}


def test_prepare_update_payload_sin_syllabus_code():
    payload = app.prepare_update_payload({"justificacion": "nueva"})
    assert payload["justificacion"] == "nueva"
    assert "syllabus_code" not in payload
    assert "fecha_modificacion" in payload


def test_prepare_update_payload_con_syllabus_code():
    code = str(uuid.uuid4())
    payload = app.prepare_update_payload({"syllabus_code": code, "justificacion": "x"})
    assert isinstance(payload["syllabus_code"], uuid.UUID)


def test_update_parcial_protege_campos_de_flujo():
    collection = _FakeCollection()
    code = uuid.uuid4()
    draft = _body(_crear_borrador(collection, code))["Data"]

    app.update_syllabus(draft["_id"], {
        "justificacion": "nueva",
        "estado": "activo",
        "flujo": {"hacked": True},
        "syllabus_code": str(code),
    }, collection)

    stored = collection.docs[0]
    assert stored["justificacion"] == "nueva"
    assert stored["estado"] == app.ESTADO_ASIGNADO
    assert "hacked" not in (stored.get("flujo") or {})
    assert str(stored["syllabus_code"]) == str(code)


def test_publicar_registra_acta_revisor_y_fechas():
    collection = _FakeCollection()
    code = uuid.uuid4()
    _crear_activo(collection, code)
    draft = _body(_crear_borrador(collection, code))["Data"]
    app.update_syllabus_estado(
        draft["_id"], {"estado": app.ESTADO_EN_REVISION, "tercero_id": 99}, collection)

    published = _body(app.publicar_syllabus(draft["_id"], {
        "tercero_id": 88,
        "nombre_aprobador": "Coordinador Z",
        "numeroActa": "ACTA-001",
        "archivo": "uuid-doc-123",
    }, collection))["Data"]

    seguimiento = published["seguimiento"]
    assert seguimiento["reviso"] == "Coordinador Z"
    assert seguimiento["aprobo"] == "Coordinador Z"
    assert seguimiento["numeroActa"] == "ACTA-001"
    assert seguimiento["archivo"] == "uuid-doc-123"
    assert seguimiento["fechaAprobacionConsejo"]


def test_reasignacion_conserva_contenido_y_propone_nuevo_docente():
    collection = _FakeCollection()
    code = uuid.uuid4()
    _crear_activo(collection, code)
    draft = _body(_crear_borrador(collection, code))["Data"]
    draft_id = draft["_id"]

    # El primer docente avanza contenido
    app.update_syllabus(draft_id, {"justificacion": "avance del docente 1"}, collection)

    # El coordinador reasigna a otro docente con plazo nuevo
    reasignado = _body(app.update_syllabus_estado(draft_id, {
        "estado": app.ESTADO_ASIGNADO,
        "asignado_a": 2000,
        "asignado_a_documento": "51999888",
        "fecha_limite": "2027-02-01",
        "tercero_id": 88,
        "observacion": "Reasignación de docente",
    }, collection))["Data"]

    assert reasignado["_id"] == draft_id
    assert reasignado["estado"] == app.ESTADO_ASIGNADO
    assert reasignado["justificacion"] == "avance del docente 1"
    assert reasignado["flujo"]["asignado_a"] == 2000
    assert reasignado["flujo"]["asignado_a_documento"] == "51999888"
    assert reasignado["flujo"]["fecha_limite"] == "2027-02-01"

    # El nuevo docente puede proponer sobre el contenido heredado
    propuesto = _body(app.update_syllabus_estado(
        draft_id, {"estado": app.ESTADO_EN_REVISION, "tercero_id": 2000}, collection))["Data"]
    assert propuesto["estado"] == app.ESTADO_EN_REVISION
    assert propuesto["justificacion"] == "avance del docente 1"
    # No se creó una versión nueva: sigue habiendo activo + un solo borrador
    assert len(collection.docs) == 2


if __name__ == "__main__":
    sys.exit(pytest.main([__file__, "-v"]))
