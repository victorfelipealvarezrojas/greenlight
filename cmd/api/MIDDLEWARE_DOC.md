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



Claude terminó la respuesta
no me queda muy claro esrto de verificacuion previa 


func (app *application) enableCORS(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Add("Vary", "Origin")
        // Agregue el encabezado "Vary: Access-Control-Request-Method".
        w.Header().Add("Vary", "Access-Control-Request-Method")
        origin := r.Header.Get("Origin")
        if origin != "" {
            for i := range app.config.cors.trustedOrigins {
                if origin == app.config.cors.trustedOrigins[i] {
                    w.Header().Set("Access-Control-Allow-Origin", origin)
                    // Comprobar si la solicitud tiene el método HTTP OPTIONS y contiene el
                    // Encabezado "Access-Control-Request-Method". Si es así, entonces tratamos
                    // como una solicitud de verificación previa.
                    if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
                        // Establece los encabezados de respuesta de verificación previa
                        w.Header().Set("Access-Control-Allow-Methods", "OPTIONS, PUT, PATCH, DELETE")
                        w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
                        // Escribe los encabezados junto con un estado 200 OK y regresa de
                        // el middleware sin ninguna acción adicional.
                        w.WriteHeader(http.StatusOK)
                        return
                    }
                    break
                }
            }
        }
        next.ServeHTTP(w, r)
    })
}
Mostrar más

Desentrañó mecanismo de verificación previa CORS y su implementación
Desentrañó mecanismo de verificación previa CORS y su implementación
La verificación previa (preflight) es una petición OPTIONS que el navegador envía automáticamente antes de la petición real, para preguntarle al servidor "¿me vas a permitir hacer un PUT con estos headers desde este origen?". Si el servidor responde que sí, el navegador recién entonces manda la petición verdadera. Si responde que no —o no responde con los headers correctos— el navegador la bloquea y nunca llega a ejecutarse.

El punto que hay que asir: el navegador es quien decide unilateralmente cuándo hace falta el preflight. El servidor no lo pide; solo lo recibe y debe saber responderlo.

Cuándo se dispara

Una petición cross-origin es "simple" (sin preflight) solo si cumple todo esto:

método GET, HEAD o POST;
solo headers de la lista segura;
si lleva Content-Type, debe ser application/x-www-form-urlencoded, multipart/form-data o text/plain.
Basta violar una condición para que el navegador obligue al preflight. Y Greenlight las viola de tres formas distintas: usa PUT/PATCH/DELETE, manda Authorization: Bearer ..., y su Content-Type es application/json. Cualquiera de las tres, por separado, ya fuerza la verificación previa.

Qué manda el navegador en ese OPTIONS

Origin
Access-Control-Request-Method: el método que va a usar la petición real (ej. PUT)
Access-Control-Request-Headers: los headers que va a incluir (ej. Authorization)
Qué hace este código

La condición central:

go
if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
No alcanza con que sea OPTIONS —un cliente puede mandar OPTIONS por otros motivos—. Lo que marca que es un preflight es la presencia del header Access-Control-Request-Method. Ese es el discriminador.

Cuando lo detecta, responde con el permiso:

Access-Control-Allow-Methods: qué métodos autoriza
Access-Control-Allow-Headers: qué headers autoriza
200 OK
Y el punto que probablemente es la fuente de la confusión: el return.

go
w.WriteHeader(http.StatusOK)
return   // ← corta acá; NO llama a next.ServeHTTP
El preflight es solo una consulta de permiso. No tiene cuerpo, no representa trabajo real. Si dejara pasar ese OPTIONS a los handlers de las rutas, chocaría contra un 405 Method Not Allowed (las rutas de Greenlight no tienen handler para OPTIONS). Por eso el middleware lo intercepta, responde él mismo y se detiene. La petición real —el PUT verdadero— llega después, como una petición nueva y separada, y esa sí atraviesa next.ServeHTTP.

En el flujo no-preflight (una petición real cross-origin), el break sale del loop y cae en next.ServeHTTP normalmente. Ahí no se cortó nada.

Puente desde Spring

En Spring todo esto lo hacía el framework por ello: @CrossOrigin o un CorsFilter con CorsConfiguration respondían el OPTIONS y ponían los Access-Control-Allow-* sin que ello escribiera una línea. Este middleware es exactamente ese mecanismo, pero desnudo: acá se ve que "manejar CORS" es, mecánicamente, (a) marcar el Vary, (b) validar el Origin contra una lista, y (c) cortocircuitar el OPTIONS de preflight con los headers de permiso. El framework no hacía magia; hacía esto.

dame u contenido md para agregar como docuemntacuion a mi proyecto


Estructuró documentación técnica sobre CORS en markdown
Estructuró documentación técnica sobre CORS en markdown
Listo. El documento cubre el mecanismo completo: qué resuelve CORS, cuándo el navegador dispara el preflight, el análisis línea por línea del middleware (con foco en el discriminante Access-Control-Request-Method y en el return que corta), un diagrama del flujo y el puente desde Spring.

Un ajuste que conviene revisar antes de fijarlo como documentación: la lista de Access-Control-Allow-Methods está escrita literal ("OPTIONS, PUT, PATCH, DELETE") siguiendo el original del libro. Si las rutas reales de tu proyecto exponen otros métodos, el doc debería reflejar los tuyos, no los del ejemplo. Lo mismo con trustedOrigins: el archivo lo describe en abstracto, sin fijar orígenes concretos, que es lo correcto para documentación versionada.


Cors preflight
Documento · MD 

¿Quieres recibir una notificación cuando Claude responda?





Claude es IA y puede cometer errores. Por favor, verifica nuevamente las respuestas.


Cors preflight · MD
# CORS y verificación previa (preflight)
 
Documentación del middleware `enableCORS`.
 
## Qué resuelve
 
El navegador aplica la *same-origin policy*: por defecto, JavaScript en `https://front.example` no puede leer respuestas de `https://api.example` (otro origen). CORS es el mecanismo por el cual el servidor **autoriza explícitamente** a orígenes concretos a hacer esas peticiones cross-origin. Sin los headers `Access-Control-Allow-*` correctos, el navegador ejecuta la petición pero **bloquea que el JavaScript lea la respuesta**.
 
Punto de partida: CORS es una decisión del **navegador**, no del servidor. El servidor solo declara permisos; el navegador los hace cumplir. Un cliente que no es navegador (curl, otro backend) ignora CORS por completo.
 
## La verificación previa
 
Para ciertas peticiones cross-origin, el navegador no manda la petición real de inmediato. Primero envía una petición **`OPTIONS`** —el *preflight*— para preguntar: "¿autorizas un `PUT` con el header `Authorization` desde este origen?". Solo si el servidor responde afirmativamente, el navegador manda la petición verdadera.
 
El preflight es una consulta de permiso. No tiene cuerpo, no ejecuta lógica de negocio, no representa trabajo real.
 
### Cuándo se dispara
 
Una petición cross-origin es *simple* (sin preflight) solo si cumple **todo** lo siguiente:
 
- Método `GET`, `HEAD` o `POST`.
- Únicamente headers de la lista segura (CORS-safelisted).
- Si lleva `Content-Type`, debe ser `application/x-www-form-urlencoded`, `multipart/form-data` o `text/plain`.
Violar **cualquiera** de estas condiciones fuerza el preflight. La API viola las tres de forma habitual:
 
| Condición | Cómo se viola |
|---|---|
| Método simple | Usa `PUT`, `PATCH`, `DELETE` |
| Solo headers seguros | Manda `Authorization: Bearer ...` |
| `Content-Type` simple | Usa `application/json` |
 
Cada una, por separado, ya basta.
 
### Qué envía el navegador en el preflight
 
- `Origin` — el origen que hace la petición.
- `Access-Control-Request-Method` — el método que usará la petición real (ej. `PUT`).
- `Access-Control-Request-Headers` — los headers que incluirá la petición real (ej. `Authorization`).
## El middleware, línea por línea
 
```go
func (app *application) enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		w.Header().Add("Vary", "Access-Control-Request-Method")
 
		origin := r.Header.Get("Origin")
 
		if origin != "" {
			for i := range app.config.cors.trustedOrigins {
				if origin == app.config.cors.trustedOrigins[i] {
					w.Header().Set("Access-Control-Allow-Origin", origin)
 
					if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
						w.Header().Set("Access-Control-Allow-Methods", "OPTIONS, PUT, PATCH, DELETE")
						w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
						w.WriteHeader(http.StatusOK)
						return
					}
 
					break
				}
			}
		}
 
		next.ServeHTTP(w, r)
	})
}
```
 
### Los headers `Vary`
 
```go
w.Header().Add("Vary", "Origin")
w.Header().Add("Vary", "Access-Control-Request-Method")
```
 
La respuesta cambia según el `Origin` (y según si es preflight). `Vary` le indica a cualquier caché intermedia que esos headers forman parte de la clave de caché. Sin esto, una caché podría servir la respuesta calculada para el origen A a una petición del origen B, filtrando o negando permisos incorrectamente.
 
### Validación del origen
 
```go
origin := r.Header.Get("Origin")
if origin != "" {
	for i := range app.config.cors.trustedOrigins {
		if origin == app.config.cors.trustedOrigins[i] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
```
 
Se compara el `Origin` recibido contra la lista blanca. Si coincide, se refleja ese origen exacto en `Access-Control-Allow-Origin`. Se refleja el valor recibido —no se usa `*`— porque el comodín es incompatible con peticiones que llevan credenciales.
 
### El discriminador del preflight
 
```go
if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
```
 
No alcanza con que el método sea `OPTIONS`: un cliente puede mandar `OPTIONS` por otros motivos. Lo que identifica inequívocamente a un preflight es la **presencia del header `Access-Control-Request-Method`**. Esa es la condición discriminante.
 
### La respuesta de permiso y el corte
 
```go
w.Header().Set("Access-Control-Allow-Methods", "OPTIONS, PUT, PATCH, DELETE")
w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
w.WriteHeader(http.StatusOK)
return
```
 
Se declaran los métodos y headers autorizados, se responde `200 OK` y **se corta con `return`**.
 
El `return` es la parte que suele generar confusión. El preflight se responde **acá mismo**; **no** se llama a `next.ServeHTTP`. Dos razones:
 
1. El preflight es solo una consulta de permiso. No hay nada real que ejecutar aguas abajo.
2. Si se dejara pasar el `OPTIONS` a los handlers de ruta, chocaría contra un `405 Method Not Allowed`, porque las rutas no tienen handler para `OPTIONS`.
La petición real —el `PUT` verdadero— llega **después**, como una petición HTTP nueva y separada. Esa sí atraviesa el middleware sin entrar en el bloque del preflight (no trae `Access-Control-Request-Method`), cae en `next.ServeHTTP` y se procesa normalmente.
 
### El flujo no-preflight
 
```go
break
```
 
Para una petición real cross-origin (no un preflight), el `break` sale del loop tras haber puesto `Access-Control-Allow-Origin`, y la ejecución continúa hacia `next.ServeHTTP`. Acá no se corta nada: el header de autorización ya quedó puesto y la petición sigue su curso.
 
## Flujo completo
 
```
Petición cross-origin con PUT + Authorization + JSON
│
├─ 1. Navegador: "esto no es simple" → manda OPTIONS (preflight)
│      Origin, Access-Control-Request-Method: PUT, Access-Control-Request-Headers: Authorization
│
├─ 2. Middleware: origin en lista blanca + tiene Access-Control-Request-Method
│      → responde Allow-Methods / Allow-Headers / 200 OK → return (corta)
│
├─ 3. Navegador: permiso concedido → manda el PUT real
│      Origin, Authorization, Content-Type: application/json, cuerpo
│
└─ 4. Middleware: pone Access-Control-Allow-Origin, break, next.ServeHTTP
       → el handler de ruta procesa el PUT
```
 
## Equivalente en Spring
 
En Spring este trabajo lo hace el framework de forma implícita: `@CrossOrigin` o un `CorsFilter` con `CorsConfiguration` responden el `OPTIONS` y ponen los `Access-Control-Allow-*` sin escribir una línea de esta lógica.
 
Este middleware es exactamente ese mecanismo, pero explícito. Deja ver que "manejar CORS" es, mecánicamente, tres cosas: **(a)** marcar el `Vary`, **(b)** validar el `Origin` contra una lista y reflejarlo, y **(c)** cortocircuitar el `OPTIONS` de preflight respondiendo con los headers de permiso. El framework no hacía magia; hacía esto.
 
