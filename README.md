# Plataforma Web con Go, PostgreSQL y HAProxy

Proyecto desarrollado para demostrar el uso de contenedores, balanceo de carga, persistencia de datos, HTTPS y automatización CI/CD.

## Arquitectura

```text
Cliente
   |
   | HTTP / HTTPS
   v
HAProxy
   |
   +------ app1 (Go)
   |
   +------ app2 (Go)
              |
              v
          PostgreSQL
              |
              v
Persistencia en el sistema anfitrión
```

La plataforma utiliza:

* Dos instancias de la aplicación desarrollada en Go.
* HAProxy como balanceador de carga.
* PostgreSQL como base de datos.
* Docker Compose para administrar los contenedores.
* Un certificado autofirmado para HTTPS.
* GitHub Actions para integración y despliegue continuo.

## Servicios

| Servicio   | Descripción                              |
| ---------- | ---------------------------------------- |
| `postgres` | Base de datos PostgreSQL                 |
| `app1`     | Primera instancia de la aplicación Go    |
| `app2`     | Segunda instancia de la aplicación Go    |
| `haproxy`  | Balanceador de carga y terminación HTTPS |

## Requisitos

* Docker
* Docker Compose
* Git
* OpenSSL
* Go, para ejecutar pruebas localmente

## Estructura principal

```text
.
├── cmd
│   └── api
│       └── main.go
├── haproxy
│   ├── certs
│   │   ├── cert.conf
│   │   ├── plataforma.crt
│   │   ├── plataforma.key
│   │   └── plataforma.pem
│   └── haproxy.cfg
├── secrets
│   └── db_password.txt
├── .github
│   └── workflows
│       └── ci-cd.yml
├── compose.yaml
├── Dockerfile
├── go.mod
└── go.sum
```

## Configuración

Crear el archivo `.env`:

```env
POSTGRES_DATA_DIR=/home/USUARIO/docker-data/plataforma-postgres
APP_IMAGE=plataforma-go:local
```

Crear el directorio persistente:

```bash
mkdir -p ~/docker-data/plataforma-postgres
```

Crear el archivo con la contraseña de PostgreSQL:

```bash
mkdir -p secrets
openssl rand -hex 32 > secrets/db_password.txt
chmod 700 secrets
chmod 644 secrets/db_password.txt
```

> Si PostgreSQL ya fue inicializado anteriormente, no se debe cambiar la contraseña guardada en `secrets/db_password.txt`.

## Ejecutar el proyecto

Validar la configuración:

```bash
docker compose config
```

Construir y levantar los servicios:

```bash
docker compose up -d --build
```

Ver el estado:

```bash
docker compose ps
```

Ver los logs:

```bash
docker compose logs --tail=100
```

Detener la plataforma:

```bash
docker compose down
```

## Acceso

Aplicación mediante HTTPS:

```text
https://localhost:8443
```

Redirección HTTP:

```text
http://localhost:8080
```

Estadísticas de HAProxy:

```text
http://localhost:8404/stats
```

Instancias directas:

```text
http://127.0.0.1:8081
http://127.0.0.1:8082
```

El certificado es autofirmado, por lo que el navegador mostrará una advertencia de seguridad.

Para probar con `curl`:

```bash
curl -k https://localhost:8443
```

## Health checks

Comprobar la primera instancia:

```bash
curl http://127.0.0.1:8081/health/ready
```

Comprobar la segunda instancia:

```bash
curl http://127.0.0.1:8082/health/ready
```

Comprobar mediante HAProxy:

```bash
curl -k https://localhost:8443/health/ready
```

## Probar el balanceo

```bash
for i in $(seq 1 10); do
    curl -skI https://localhost:8443/ |
        grep -i X-Backend-Instance
done
```

Las respuestas deben alternar entre los identificadores de `app1` y `app2`.

## Simular el fallo de una instancia

Detener `app1`:

```bash
docker stop plataforma-app1
```

Probar nuevamente:

```bash
curl -k https://localhost:8443
```

HAProxy debe continuar atendiendo las solicitudes mediante `app2`.

Restaurar la instancia:

```bash
docker start plataforma-app1
```

## Persistencia de PostgreSQL

Los datos se almacenan fuera del contenedor mediante un bind mount:

```text
~/docker-data/plataforma-postgres
```

Para comprobar la persistencia:

```bash
docker compose down
ls -la ~/docker-data/plataforma-postgres
docker compose up -d
```

Los datos deben conservarse después de eliminar y recrear los contenedores.

## Pruebas de Go

```bash
go test ./...
```

```bash
go vet ./...
```

## CI/CD

El archivo:

```text
.github/workflows/ci-cd.yml
```

automatiza:

1. Descarga del repositorio.
2. Ejecución de pruebas.
3. Ejecución de `go vet`.
4. Construcción de la imagen Docker.
5. Publicación de la imagen en GitHub Container Registry.
6. Actualización de `app1` y `app2` mediante Docker Compose.
7. Verificación del endpoint `/health/ready`.

El flujo se ejecuta al realizar un `push` a la rama `main`.

## Seguridad aplicada

* Aplicaciones ejecutadas con usuario no privilegiado.
* Sistema de archivos de las aplicaciones en modo de solo lectura.
* Eliminación de capacidades Linux innecesarias.
* Uso de `no-new-privileges`.
* PostgreSQL no expone puertos directamente.
* La contraseña se carga desde un archivo de secreto.
* HTTPS mediante certificado autofirmado.
* HAProxy es el único punto de entrada público.

## Limitaciones

La solución utiliza un único Docker Engine, un único HAProxy y una única instancia de PostgreSQL.

Por lo tanto, demuestra tolerancia al fallo de una instancia Go, pero no alta disponibilidad completa de toda la infraestructura.


Sistemas Operativos II
