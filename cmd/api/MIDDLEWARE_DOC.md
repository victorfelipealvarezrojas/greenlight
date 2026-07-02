## recoverPanic — Middleware de recuperación de pánicos

`recoverPanic` envuelve toda la cadena de ejecución de la request usando un `defer` con `recover()`.

- Captura panics inesperados que ocurran en cualquier middleware o handler siguiente.
- Si ocurre un panic, fuerza el header `Connection: close` y responde con un error 500 interno.
- Esto evita que la goroutine termine silenciosamente sin responder al cliente.
- `recover()` solo atrapa panics; los errores normales de Go (`err != nil`) deben manejarse como valores de retorno.

el recover automático de net/http es genérico y básico — cierra la conexión abruptamente sin darle al cliente una respuesta HTTP estructurada. 
Lo que el cliente recibe, sin el middleware, es una conexión cortada de golpe (posiblemente un error de conexión reset, sin cuerpo, sin código 
de estado claro) — una experiencia fea e inconsistente con el resto de la API.

En `routes.go` el orden importa:

- `app.rateLimitWithIP(router)` se monta primero.
- `app.recoverPanic(...)` envuelve ese resultado.

Por eso, un panic dentro del middleware de limitación de velocidad también queda atrapado por `recoverPanic`.

---

## Rate Limiting — Middleware de limitación de velocidad

### Librería
`golang.org/x/time/rate` — paquete extendido de Go para control de tasas.
No es stdlib — requiere instalación:

```bash
go get golang.org/x/time/rate
```

---

## v1 — Global (deprecado)

> Un único limiter compartido por **todos** los clientes. Útil como punto de partida, no para producción con múltiples usuarios.

### Implementación

```go
// rateLimitv01 limita las solicitudes con un único limiter global.
//
// Deprecated: usa rateLimit, que aplica el límite por IP.
func (app *application) rateLimitv01(next http.Handler) http.Handler {
    limiter := rate.NewLimiter(2, 4)

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !limiter.Allow() {
            app.rateLimitExceededResponse(w, r)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

### Algoritmo — Token Bucket
- `rate.NewLimiter(2, 4)` — 2 tokens/segundo de recarga, burst máximo de 4
- Cada request consume un token
- Sin tokens disponibles → 429 Too Many Requests

### Closure
`limiter` se inicializa una sola vez al montar el middleware (en `routes()`, antes de que exista ninguna request).
La función anónima retornada captura `limiter` del scope externo — lo mantiene vivo entre requests compartiendo el mismo estado.
Sin el closure, `limiter` se reiniciaría en cada request y el rate limiting no funcionaría.

### Por qué es seguro entre goroutines
Cada request corre en su propia goroutine, pero todas comparten el mismo `*rate.Limiter` vía closure.
`rate.Limiter` trae su propio `mutex` interno — el read-modify-write de tokens (leer disponibles, reponer por tiempo, restar) ocurre bajo ese lock. Por eso compartir una sola instancia entre goroutines no es data race: la sincronización viene incluida en el tipo, no la pones tú.

### Limitación
El límite es **global**: todos los clientes compiten por los mismos 4 tokens de ráfaga. Un solo cliente agresivo puede agotarlos para todos. De ahí la v2.

---

## v2 — Por IP

> Un limiter independiente por cliente (IP). Cada IP tiene su propio bucket de tokens; uno no afecta a los demás.

### Implementación

```go
func (app *application) rateLimitWithIP(next http.Handler) http.Handler {

    type client struct {
        limiter  *rate.Limiter
        lastSeen time.Time
    }

    var (
        mu      sync.Mutex
        clients = make(map[string]*client)
    )

    // Goroutine de limpieza — vive tanto como el proceso.
    go func() {
        for {
            time.Sleep(time.Minute)
            mu.Lock()
            for ip, client := range clients {
                if time.Since(client.lastSeen) > 3*time.Minute {
                    delete(clients, ip)
                }
            }
            mu.Unlock()
        }
    }()

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

        ip, _, err := net.SplitHostPort(r.RemoteAddr)
        if err != nil {
            app.serverErrorResponse(w, r, err)
            return
        }

        mu.Lock()

        if _, found := clients[ip]; !found {
            clients[ip] = &client{limiter: rate.NewLimiter(2, 4)}
        }

        clients[ip].lastSeen = time.Now()

        if !clients[ip].limiter.Allow() {
            mu.Unlock()
            app.rateLimitExceededResponse(w, r)
            return
        }

        mu.Unlock()

        next.ServeHTTP(w, r)
    })
}
```

### Qué cambia respecto a v1
Un `map[string]*client` reemplaza al `*rate.Limiter` único. Cada IP recibe su propio limiter (2 tokens/seg, burst 4) la primera vez que se le ve.

### Mutex: responsabilidad propia, no heredada
`rate.Limiter` sigue trayendo su mutex interno (igual que en v1) — `Allow()` es seguro por sí solo.
Pero **el `map` nativo de Go no es thread-safe**. Leer y escribir el mapa concurrentemente (`clients[ip]` para buscar/crear) sin protección es data race garantizado — el runtime lo detecta como panic de "concurrent map read and map write".
Por eso el `sync.Mutex` acá es tuyo: protege el acceso al mapa, no al limiter individual. Cada candado protege lo que le corresponde — el del limiter viene incluido, el del mapa lo pones tú.

### `lastSeen` — no es parte del algoritmo de rate limiting
`clients[ip].lastSeen = time.Now()` se actualiza en cada request, pero el `Allow()` del limiter nunca lo lee ni lo usa. Son dos relojes distintos que conviven en el mismo struct por economía de acceso (una sola búsqueda en el mapa), no por necesidad lógica:

- **Reloj del limiter** (interno, privado a `rate.Limiter`) — decide si la request pasa. Esto *es* el rate limiting.
- **Reloj de `lastSeen`** (gestionado a mano) — decide cuándo un cliente se considera inactivo para limpieza del mapa. Es housekeeping, no decisión de tráfico.

Si se eliminara `lastSeen` por completo, el rate limiting seguiría funcionando igual; solo la limpieza del mapa dejaría de tener criterio.

### La goroutine de limpieza
- Se crea **una sola vez**, al evaluar `rateLimitWithIP(router)` al montar las rutas — antes de que exista ninguna request.
- `for {}` sin condición de salida: vive **mientras viva el proceso**, no mientras existan solicitudes. Si el servidor está idle, igual despierta cada minuto a revisar un mapa vacío.
- Es segura sin mecanismo de cancelación porque su dueño (el middleware) vive tanto como el proceso. Si se lanzara una goroutine así por entidad de vida corta (por request, por conexión), sería fuga de goroutines — se acumulan, retienen lo que su closure referencia, y eventualmente el proceso muere por OOM (no por panic; la fuga es silenciosa).
- Muere junto con todo el programa cuando `main` retorna — el runtime descarta todas las goroutines vivas en ese momento, sin ejecutar sus `defer`.

### Por qué `Allow()` se llama dentro del lock del mapa aquí
En esta versión, `Allow()` ocurre mientras `mu` sigue tomado (recién se libera después, en el branch de éxito, o antes de la respuesta 429). Funciona porque el lock es reentrante-seguro en el sentido de que un solo goroutine lo sostiene de punta a punta — no hay deadlock. El costo es contención: todas las requests, de cualquier IP, se serializan en ese único `mu` aunque cada una tenga su propio limiter. Una variante de menor contención saca `Allow()` fuera del lock del mapa, ya que el limiter se protege solo:

```go
mu.Lock()
c, found := clients[ip]
if !found {
    c = &client{limiter: rate.NewLimiter(2, 4)}
    clients[ip] = c
}
c.lastSeen = time.Now()
mu.Unlock()

if !c.limiter.Allow() {
    app.rateLimitExceededResponse(w, r)
    return
}
```

### Limitación de v2
El límite ahora es por IP, pero el mapa crece sin tope superior salvo por la limpieza cada minuto — bajo un ataque de IPs falsificadas (`X-Forwarded-For` sin validar, o `RemoteAddr` spoofeado en entornos sin proxy de confianza) el mapa puede crecer agresivamente entre limpiezas.


