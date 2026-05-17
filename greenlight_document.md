# Greenlight — Herramientas y dependencias globales

Documentación de las herramientas externas al proyecto. No son parte del código fuente pero son requisito para desarrollar, testear y desplegar la API.

---

## curl

**Qué es:** Cliente HTTP de línea de comandos. Permite hacer requests HTTP manualmente desde el terminal.

**Por qué se usa:** Para interactuar con los endpoints de la API durante el desarrollo sin necesidad de un cliente gráfico. Permite inspeccionar headers, bodies, códigos de respuesta y probar autenticación directamente.

**Instalación:** Pre-instalado en macOS y Linux. En Windows descargar desde https://curl.se.

**Ejemplo típico en este proyecto:**
```bash
curl -H "Authorization: Bearer RIDBIAE3AMMK57T6IAEBUGA7ZQ" localhost:4000/v1/movies/1
```

---

## hey

**Qué es:** Herramienta de load testing para HTTP. Hace múltiples requests concurrentes contra un endpoint y reporta métricas de rendimiento.

**Por qué se usa:** Para medir el comportamiento de la API bajo carga. Útil cuando se implementa rate limiting, para verificar que los límites funcionan correctamente, y para medir el impacto de optimizaciones.

**Instalación:**
```bash
go install github.com/rakyll/hey@latest
```

**Métricas que reporta:**
- Requests por segundo
- Latencia promedio
- Percentiles p50, p95, p99
- Distribución de códigos de respuesta

**Ejemplo típico:**
```bash
hey -n 1000 -c 100 localhost:4000/v1/healthcheck
# -n: total de requests
# -c: requests concurrentes
```

---

## git

**Qué es:** Sistema de control de versiones.

**Por qué se usa:** Control de cambios del proyecto. Requerido también para el proceso de deployment al servidor en Digital Ocean descrito en el libro.

**Instalación:** https://git-scm.com/downloads

---

## PostgreSQL

**Qué es:** Base de datos relacional.

**Por qué se usa:** Almacenamiento persistente de todos los datos de la API (películas, usuarios, tokens). El libro usa PostgreSQL específicamente por su soporte de tipos avanzados y porque es el estándar en proyectos Go serios.

**Notas:** El libro asume una instancia local durante desarrollo. Las credenciales y DSN se manejan via flags de línea de comandos, no hardcodeados.

---

## httprouter

**Qué es:** Paquete externo de Go para routing HTTP. Alternativa al `net/http` estándar con soporte de parámetros en rutas y mejor rendimiento.

**Por qué se usa:** El router estándar de Go no soporta parámetros en rutas (`:id`) ni diferenciación por método HTTP de forma limpia. `httprouter` agrega eso con overhead mínimo.

**Nota:** desde Go 1.22 el router estándar (`net/http`) soporta parámetros en rutas y diferenciación por método HTTP, reduciendo la necesidad de httprouter para proyectos simples,
pero la limitación de 404/405 en texto plano persiste

**Instalación:**
```bash
go get github.com/julienschmidt/httprouter@v1 
```

**Ejemplo:**
```go
router.HandlerFunc(http.MethodGet, "/v1/movies/:id", app.showMovieHandler)
```

---

## Estructura del proyecto

```
greenlight/
├── bin/          # Binarios compilados para deployment
├── cmd/
│   └── api/
│       └── main.go   # Entrypoint, wiring de dependencias
├── internal/     # Paquetes internos: DB, validación, mail, etc.
│                 # Solo importables desde dentro del proyecto (restricción Go)
├── migrations/   # Archivos SQL de migraciones de base de datos
├── remote/       # Configuración y scripts para servidor de producción
├── go.mod        # Dependencias y versiones exactas
└── Makefile      # Tareas automatizadas: build, audit, migrations
```

### Por qué `internal/` es especial

En Go el directorio `internal` tiene comportamiento reservado por el lenguaje: los paquetes dentro de él solo pueden ser importados por código dentro del directorio padre. Esto impide que código externo (otros proyectos, otras personas) importe paquetes que no están pensados para uso público, aunque el repositorio sea abierto.

---

## Makefile

**Qué es:** Archivo de recetas para automatizar tareas comunes del proyecto.

**Por qué se usa:** Evita tener que recordar comandos largos. Centraliza tareas como compilar, correr el servidor, ejecutar migraciones, auditar dependencias.

**Tareas típicas en este proyecto:**
```makefile
make run/api       # Levanta el servidor
make db/migrations/up  # Ejecuta migraciones pendientes
make audit         # Verifica dependencias y corre go vet
make build/api     # Compila el binario para producción
```

---

## Digital Ocean

**Qué es:** Proveedor de servidores cloud.

**Por qué se usa:** El libro despliega la API en un servidor Linux en Digital Ocean al final. No es requisito durante desarrollo, solo para el capítulo de deployment.

---

*Actualizar este documento a medida que el libro introduzca nuevas herramientas o dependencias.*