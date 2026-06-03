## Rate Limiting — Middleware de limitación de velocidad

### Librería
`golang.org/x/time/rate` — paquete extendido de Go para control de tasas.
No es stdlib — requiere instalación:

```bash
go get golang.org/x/time/rate
```

### Implementación

```go
func (app *application) rateLimit(next http.Handler) http.Handler {
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
`limiter` se inicializa una sola vez al montar el middleware.
La función anónima retornada captura `limiter` del scope externo —
lo mantiene vivo entre requests compartiendo el mismo estado.
Sin el closure, `limiter` se reiniciaría en cada request y el rate limiting no funcionaría.