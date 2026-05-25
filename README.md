# Greenlight

JSON REST API for retrieving and managing information about movies.

Built with Go following [Let's Go Further](https://lets-go-further.alexedwards.net/) by Alex Edwards.

## Stack

- Go 1.23
- PostgreSQL
- httprouter (se utiliza el paquete de terceros httprouter como enrutador, en lugar de usar http.ServeMux de la biblioteca estándar.)

## Project Structure

```
greenlight/
├── bin/          # Compiled binaries
├── cmd/
│   └── api/
│       ├── errors.go
│       ├── healthcheck.go
│       ├── helpers.go
│       ├── main.go
│       ├── middleware.go
│       ├── movies.go
│       └── routes.go
├── internal/     # Internal packages
│   └── data/
│       ├── movies.go
│       └── runtime.go
├── migrations/   # SQL migration files
├── remote/       # Production server configuration
├── go.mod
├── go.sum
└── Makefile
```

## Tools

- `curl` — Manual HTTP testing
- `hey` — Load testing
- `git` — Version control

## Setup

```bash
    git clone https://github.com/valvarez/greenlight
    cd greenlight
    go mod download
```

## Ejecución

### Ejecutar con configuración por defecto

- `go run ./cmd/api`
- Arranca el servidor en el puerto `4000`
- Usa el entorno `development`

### Ejecutar con parámetros

- `go run ./cmd/api -port=8080 -env=production`
- Reemplaza `8080` por el puerto deseado
- Usa el entorno `production`, `staging` o `development`

> Nota: los flags disponibles son `-port` y `-env`.

## Endpoints

**Health**
| Method | URL | Action |
|--------|-----|--------|
| GET | /v1/healthcheck | Show application information |

**Movies**
| Method | URL | Action |
|--------|-----|--------|
| POST | /v1/movies | Create a new movie |
| GET | /v1/movies/:id | Show a specific movie |


## Progress

# Docker + PostgreSQL — Referencia rápida

## docker-compose.yml

```yaml
services:
  postgres:
    image: postgres:latest
    container_name: progress-db        # nombre fijo y predecible del container
    environment:
      POSTGRES_DB: greenlight          # nombre de la base de datos
      POSTGRES_USER: admin             # usuario de PostgreSQL
      POSTGRES_PASSWORD: pass12345     # contraseña
    ports:
      - "5432:5432"                    # puerto host:container
    volumes:
      - db-data:/var/lib/postgresql

volumes:
  db-data:
```

---

## Levantar y detener

```bash
docker-compose up -d       # levanta los containers en background
docker-compose down        # detiene y elimina los containers
docker-compose down -v     # detiene y elimina containers + volúmenes (borra datos)
```

---

## Conectarse a PostgreSQL

```bash
# conectarse a la db definida en el compose
docker exec -it progress-db psql -U admin -d greenlight
```

- `progress-db` — siempre es el `container_name` definido en el compose
- `-U admin` — usuario definido en `POSTGRES_USER`
- `-d greenlight` — db definida en `POSTGRES_DB`

Si Docker no tiene `container_name` explícito genera uno automático tipo `greenlight-postgres-1` — por eso conviene definirlo siempre.

---

## Configuración inicial de la base de datos

Conectado a psql, ejecutar en orden:

```sql
-- usuario con permisos limitados solo a greenlight (buena práctica — no usar el superusuario)
CREATE ROLE greenlight WITH LOGIN PASSWORD 'pa55word';

-- extensión para strings case-insensitive (necesaria para emails de usuarios)
CREATE EXTENSION IF NOT EXISTS citext;
```

### Verificar

```sql
\du   -- lista roles — debe aparecer greenlight sin atributos de superusuario
\dx   -- lista extensiones — debe aparecer citext
```

### citext
Agrega un tipo de dato que ignora mayúsculas al comparar. Sin él, `user@gmail.com` y `User@Gmail.com` serían distintos — con él son iguales. Se usa en la columna de email para evitar registros duplicados por diferencia de capitalización.


## Comandos dentro de psql

```sql
\l              -- listar bases de datos
\c nombre_db    -- conectarse a una db
\dt             -- listar tablas
\d nombre_tabla -- describir una tabla
\q              -- salir
\du             -- lista roles — debe aparecer greenlight sin atributos de superusuario
\dx             -- lista extensiones — debe aparecer citext
```

---

## Comandos útiles sin entrar a psql

```bash
# ejecutar un comando sin sesión interactiva
docker exec progress-db cat /etc/passwd | grep 'postgres'

# verificar que el container está corriendo
docker ps

# ver logs del container
docker logs progress-db
```

---

## Usuarios Linux en el container

```bash
cat /etc/passwd | grep 'postgres'
# postgres:x:999:999::/var/lib/postgresql:/bin/bash
#           ↑   ↑
#          uid  gid
```

- **uid/gid** — identificadores numéricos del usuario en el sistema Linux del container
- Los servicios (postgres, redis, nginx) usan el rango **100-999**
- Los usuarios humanos usan desde **1000** en adelante
- No tiene relevancia para el uso de PostgreSQL — es información del SO del container

El usuario del sistema `postgres` es distinto al usuario de la DB definido en `POSTGRES_USER`.


## DSN de conexión desde Go

```
postgres://greenlight:pa55word@localhost/greenlight?sslmode=disable
```

## Configurar Variable d entorno DSN

## En Mac el shell por defecto es `zsh` — el archivo equivalente es `~/.zshrc`:

```bash
echo 'export GREENLIGHT_DB_DSN="postgres://greenlight:pa55word@localhost/greenlight?sslmode=disable"' >> ~/.zshrc
```

### recargas archivo para que tome efecto en la sesión actual:

```bash
source ~/.zshrc
```

### Verificas que quedó:

```bash
echo $GREENLIGHT_DB_DSN
```



# Estructura:

```
postgres:// greenlight : pa55word @ localhost / greenlight ? sslmode=disable
            ↑             ↑           ↑           ↑
         POSTGRES_USER  password    host        POSTGRES_DB
```


| Chapter | Topic | Status |
|---------|-------|--------|
| 2.1 | Project setup and skeleton structure                  | ✅ |
| 2.2 | A Basic HTTP Server.                                  | ✅ |
| 2.3 | API Endpoints and RESTful Routing                     | ✅ |
| 3.  | Sending JSON Responses & Fixed-Format JSON            | ✅ |
| 3.2 | JSON Encoding                                         | ✅ |
| 3.3 | Encoding Structs                                      | ✅ |
| 3.4 | Formatting and Enveloping Responses                   | ✅ |
| 3.5 | Advanced JSON Customization                           | ✅ |
| 3.6 | Sending Error Messages                                | ✅ |
| 4.0 | Parsing JSON Requests                                 | ✅ |
| 4.1 | JSON Decoding                                         | ✅ |
| 4.2 | Managing Bad Requests                                 | ✅ |
| 4.3 | Restricting Inputs.                                   | ✅ |
| 4.4 | Custom JSON Decoding.                                 | ✅ |
| 4.5 | Validating JSON Input.                                | ✅ |
| 5   | Database Setup and Configuration                      | ✅ |
| 5.1 | Setting up PostgreSQL                                 | ✅ |
| 5.2 | Connecting to PostgreSQL                              | ✅ |
```

