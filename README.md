# Syllabus Lambda (Prueba Piloto)

Pruebas piloto CRUD del syllabus empleando AWS SAM.

## Especificaciones Técnicas

### Tecnologías Implementadas y Versiones
* [Python 3.11](https://docs.python.org/3.11/) (runtime de las Lambdas; el CI usa 3.13)
* [AWS SAM](https://docs.aws.amazon.com/es_es/serverless-application-model/latest/developerguide/using-sam-cli.html)
* [AWS SAM CLI](https://docs.aws.amazon.com/es_es/serverless-application-model/latest/developerguide/install-sam-cli.html)
* Opcional (Requerido para ejecutar el servicio API en local, simula el API Gateway) [Docker](https://docs.docker.com/engine/install/ubuntu/)
* Opcional (Gestión de versiones de librerias en local) [virtualenv](https://virtualenv.pypa.io/en/latest/installation.html)


### Variables de Entorno
```shell
SYLLABUS_CRUD_HOST=[direccion de la base de datos]
SYLLABUS_CRUD_PORT=[Puerto de conexión con la base de datos]
SYLLABUS_CRUD_USERNAME=[usuario con acceso a la base de datos]
SYLLABUS_CRUD_PASS=[password del usuario]
SYLLABUS_CRUD_DB=[nombre de la base de datos]
TIMEZONE=[zona horaria]
```

**Nota:**
* Por defecto se asignó "America/Bogota", para ver más opciones vea [Lista de zona horarias](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones)


### Ejecución del Proyecto en Local
```shell
sam build
sam local start-api --env-vars env.json
```
**Nota:**
* Para más detalle de las formas de ejecutarlo localmente vea [Uso sam local](https://docs.aws.amazon.com/es_es/serverless-application-model/latest/developerguide/using-sam-cli-local.html)
* Puede usar el script `run_local.sh` para correr los comandos indicados anteriormente con bash. 

### Ejecución Pruebas

`tests/requirements.txt` incluye las dependencias de los handlers involucrados
(CRUD y `html_2_pdf`). Suite completa:

```shell
pip install -r tests/requirements.txt
pytest tests/
```

Solo flujo de aprobación (sin dependencias de PDF HTML):

```shell
pytest tests/test_flujo_aprobacion.py
```

Pruebas del generador de PDF (Go, junto al handler por convención de paquete):

```shell
cd src/handlers/gofpdf_syllabus && go test ./...
```

### Endpoints expuestos

* `POST /syllabus`, `GET /syllabus`, `GET /syllabus/{code}`, `PUT /syllabus/{id}`, `DELETE /syllabus/{id}`
* `PUT /syllabus/{id}/estado`, `PUT /syllabus/{id}/publicar`
* `POST /v2/syllabus/template` (generación de PDF, gofpdf)
* `POST /v3/syllabus/template` (generación de PDF desde HTML)
* La generación XLSX fue retirada. No se expone endpoint `/health`; los
  despliegues se validan con `sam validate` y pruebas sobre los endpoints reales.

### Flujo de aprobación

El syllabus incorpora un flujo de aprobación modelado sobre el mismo documento
(colección `syllabus`), agrupado por `syllabus_code`:

* `estado`: `activo | asignado | en_revision | rechazado | expirado | cancelado`.
  Los estados distintos de `activo` corresponden a propuestas no publicadas y no
  se devuelven en las consultas por defecto.
* `flujo`: metadata de la asignación (`asignado_a`, `asignado_a_documento`,
  `asignado_por`, `asignado_por_documento`, `fecha_limite`, `historial`, ...).

Endpoints del flujo:

| Método | Ruta | Descripción |
| -- | -- | -- |
| PUT | `/syllabus/{id}/estado` | Aplica una transición de estado validada |
| PUT | `/syllabus/{id}/publicar` | Publica (aprueba) la propuesta como versión actual |

Un borrador (creado con `estado` distinto de `activo`) no despublica la versión
vigente; solo `/publicar` intercambia la versión actual.

`PUT /syllabus/{id}` es una **actualización parcial**: solo modifica los campos
enviados. Los campos de flujo/versionado (`syllabus_code`, `version`,
`syllabus_actual`, `estado`, `flujo`, `creado_por`) son inmutables y se
descartan aunque vengan en el cuerpo.

Los documentos previos al flujo no tienen el campo `estado`; se tratan como
`activo` (retrocompatible) y adquieren su `estado` a medida que se editan o
publican.

### Manejo de fechas

* El CRUD almacena los snapshots del flujo (`fecha_asignacion`,
  `flujo.*`) en ISO 8601 con offset Bogotá (`-05:00`, vía `TIMEZONE`) y los
  datetimes BSON (`fecha_creacion`) en UTC (Mongo).
* La conversión a la fecha visible es responsabilidad del consumidor.

### Despliegue
```shell
sam build
sam deploy --guided
```
**Nota:** 
* Para mayor información para realizar el despliegue vea [Uso sam deploy](https://docs.aws.amazon.com/es_es/serverless-application-model/latest/developerguide/using-sam-cli-deploy.html).

## Estado CI


## Licencia
