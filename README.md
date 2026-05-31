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
├── bin/                     # Compiled binaries
├── cmd/
│   └── api/
│       ├── errors.go
│       ├── healthcheck.go
│       ├── helpers.go
│       ├── main.go
│       ├── middleware.go
│       ├── movies.go
│       └── routes.go
├── internal/                # Internal packages
│   ├── data/
│   │   ├── models.go
│   │   ├── movies.go
│   │   └── runtime.go
│   └── validator/
│       └── validator.go
├── migrations/              # SQL migration files
├── remote/                  # Production server configuration
├── docker-compose.yml       # Docker Compose configuration
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
| PUT | /v1/movies/:id | Update an existing movie |
| DELETE | /v1/movies/:id | Delete a movie |


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

````markdown
## Configuración inicial de la base de datos

Conectado a psql con `admin`, ejecutar en orden:

```sql
-- usuario con permisos limitados solo a greenlight (buena práctica — no usar el superusuario)
CREATE ROLE greenlight WITH LOGIN PASSWORD 'pa55word';

-- extensión para strings case-insensitive (necesaria para emails de usuarios)
CREATE EXTENSION IF NOT EXISTS citext;

-- puede ser necesario configurarlo como propietario
ALTER DATABASE greenlight OWNER TO greenlight;

-- si no funciono el paso anterior utilzar permiso especifico 
GRANT CREATE ON DATABASE greenlight TO greenlight;

-- ******en caso de que los 2 passo anteriores no funciones, podria ser ademas necesario pero requiere un admin y cambair de usuario****
-- permisos sobre el schema public — necesario para que greenlight pueda crear tablas
-- debe ejecutarse con admin, no con greenlight asi que en este punto cambair al user admin
GRANT ALL ON SCHEMA public TO greenlight;
```

### Verificar

```sql
\du   -- lista roles — debe aparecer greenlight sin atributos de superusuario
\dx   -- lista extensiones — debe aparecer citext
```

### citext
Agrega un tipo de dato que ignora mayúsculas al comparar. Sin él, `user@gmail.com` y `User@Gmail.com` serían distintos — con él son iguales. Se usa en la columna de email para evitar registros duplicados por diferencia de capitalización.

---

## Comandos psql

```bash
# conectarse con superusuario (para operaciones de administración)
docker exec -it progress-db psql -U admin -d greenlight

# conectarse con usuario limitado (uso normal)
docker exec -it progress-db psql -U greenlight -d greenlight
```

```sql
\l              -- listar bases de datos
\c nombre_db    -- conectarse a una db
\dt             -- listar tablas
\d nombre_tabla -- describir una tabla
\q              -- salir
\du             -- listar roles
\dx             -- listar extensiones
\d movies       -- estructura de la tabla
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

## Connection Pool — Métodos de configuración

### SetMaxOpenConns
Límite máximo de conexiones abiertas (en uso + idle) en el pool.
Por defecto ilimitado. Si se alcanza el límite, las nuevas consultas esperan
hasta que una conexión quede disponible.

### SetMaxIdleConns
Límite máximo de conexiones idle (abiertas pero sin usar) que el pool mantiene.
Por defecto 2. Conexiones idle por encima del límite se cierran automáticamente.

### SetConnMaxLifetime
Tiempo máximo de vida de una conexión — cuánto tiempo puede existir
antes de ser cerrada y reemplazada, independiente de si está en uso o idle.
Por defecto ilimitado.

### SetConnMaxIdleTime
Tiempo máximo que una conexión puede estar idle antes de ser cerrada.
Por defecto ilimitado. Útil para liberar conexiones en períodos de baja carga.

---

**Regla general para producción:**
```go
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(25)
db.SetConnMaxLifetime(5 * time.Minute)
db.SetConnMaxIdleTime(5 * time.Minute)
```
Los valores óptimos dependen del hardware y la carga — requieren benchmarking.

go run ./cmd/api -db-max-open-conns=50 -db-max-idle-conns=50 -db-max-idle-time=2h30m


## Migración

### Concepto

Cada cambio al schema de la DB se representa como un par de archivos numerados secuencialmente:

```
000001_create_movies_table.up.sql    → aplica el cambio
000001_create_movies_table.down.sql  → revierte el cambio
```

El tool de migración registra qué migraciones ya se aplicaron — solo ejecuta las pendientes.

**Ventajas:**
- El schema vive en el repositorio junto al código
- Se puede replicar el schema exacto en cualquier entorno
- Se puede hacer rollback de cualquier cambio

---

### Instalación — migrate tool

**macOS:**
```bash
brew install golang-migrate
```

**Linux:**
```bash
cd /tmp
curl -L https://github.com/golang-migrate/migrate/releases/download/v4.16.2/migrate.linux-amd64.tar.gz | tar xvz
mv migrate ~/go/bin/
```

**Verificar instalación:**
```bash
migrate -version
```

### Crear archivos de migración

```bash
migrate create -seq -ext=.sql -dir=./migrations nombre_migracion
# genera:
# migrations/000001_nombre_migracion.up.sql
# migrations/000001_nombre_migracion.down.sql

# flags:
# -seq  → numeración secuencial (0001, 0002...) en lugar de Unix timestamp
# -ext  → extensión de los archivos generados
# -dir  → directorio donde se guardan (se crea automáticamente si no existe)
# nombre_migracion → label descriptivo que indica el contenido
```

**Regla importante:** cada migración contiene solo el cambio incremental — nunca repite
cambios de migraciones anteriores. El tool aplica los archivos en orden secuencial
y registra cuáles ya ejecutó, saltándolos en ejecuciones posteriores.

```
000001_create_movies_table       → crea tabla movies
000002_add_movies_check_constraints → agrega constraints a movies
000003_add_users_table           → crea tabla users (no repite movies)
```

Cambio nuevo → migración nueva. Nunca se modifica una migración ya aplicada.
````

### Ejecutar migraciones

```bash
# aplicar todas las pendientes (GREENLIGHT_DB_DSN es la credencial que fue agregada a las variables de entorno)
migrate -path=./migrations -database=$GREENLIGHT_DB_DSN up

# revertir la última
migrate -path=./migrations -database=$GREENLIGHT_DB_DSN down 1
```
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
| 5.3 | Configuring the Database Connection                   | ✅ |
| 6.  | SQL Migrations                                        | ✅ |
| 6.1 | An Overview of SQL Migrations                         | ✅ |
| 6.2 | Working with SQL Migrations                           | ✅ |
| 7.  | CRUD Operations                                       | ✅ |
| 7.1 | Setting up the Movie Model                            | ✅ |
| 7.2 | Creating a New Movie                                  | ✅ |
| 7.3 | Fetching a Movie                                      | ✅ |
| 7.3 | Updating a Movie                                      | ✅ |
| 7.4 | Deleting a Movie                                      | ✅ |
| 8.  | Advanced CRUD Operations                              | ✅ |
| 8.1 | Handling Partial Updates                              | ✅ |
| 8.2 | Optimistic Concurrency Control                        | ✅ |
| 8.3 | Managing SQL Query Timeouts                           | ✅ |
| 8.4 | Filtering, Sorting, and Pagination                    | ✅ |
| 8.4 | Filtering, Sorting, and Pagination                    | ✅ |
| 9.  | Parsing Query String Parameters.                      | ✅ |
| 9.4 | Filtering List                                        | ✅ |
```

