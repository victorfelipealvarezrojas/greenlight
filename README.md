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
│       └── main.go
│       └── healthcheck.go
├── internal/     # Internal packages (DB, validation, mail)
├── migrations/   # SQL migration files
├── remote/       # Production server configuration
├── go.mod
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

| Chapter | Topic | Status |
|---------|-------|--------|
| 2.1 | Project setup and skeleton structure                  | ✅ |
| 2.2 | A Basic HTTP Server.                                  | ✅ |
| 2.3 | API Endpoints and RESTful Routing                     | ✅ |
| 3.  | Sending JSON Responses & Fixed-Format JSON            | ✅ |
| 3.2 | JSON Encoding                                         | ✅ |
| 3.3 | Encoding Structs                                      | ✅ |
```

