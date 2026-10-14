# Syllabus Lambda (Prueba Piloto)

Pruebas piloto CRUD del syllabus empleando AWS SAM.

## Especificaciones Técnicas

### Tecnologías Implementadas y Versiones
* [Python 3.9](https://docs.python.org/3.9/)
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

Pruebas unitarias de la máquina de estados del flujo de aprobación (pytest):

```shell
pip install -r tests/requirements.txt
pytest tests/
```

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

### Despliegue
```shell
sam build
sam deploy --guided
```
**Nota:** 
* Para mayor información para realizar el despliegue vea [Uso sam deploy](https://docs.aws.amazon.com/es_es/serverless-application-model/latest/developerguide/using-sam-cli-deploy.html).

## Estado CI


## Licencia
