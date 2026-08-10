# Spec hijo — Emisión de token de autenticación (HTTP + Go, Greenlight)

> **Nivel:** hijo. Concreta el padre [`../generacion-token-autenticacion.md`](../generacion-token-autenticacion.md)
> para este proyecto: Go, `httprouter`, PostgreSQL, endpoint HTTP JSON.
> No repite el "por qué" — solo cierra lo que el padre dejó abierto y referencia
> la regla (R1–R6) que cada decisión satisface.

---

## Decisiones cerradas

| Abierto en el padre | Cierre en este hijo |
|---|---|
| Persistencia | **Stateful.** Token opaco, hash SHA-256 persistido en tabla `tokens`. Reutiliza `data.TokenModel` (`internal/data/tokens.go`), ya usado para el scope `activation`. Revocación = `DELETE` por scope+user (`DeleteAllForUser`, ya existe). |
| Expiración | **24 horas** desde la emisión. |
| Forma de la credencial | Identificador = `email`. Secreto = `password`. Verificación con `bcrypt` vía `user.Password.Matches(...)` (`internal/data/users.go`), ya es la comparación costosa que exige R1. |
| Precondición de cuenta | **Se exige `Activated == true`.** Si el email y el password son correctos pero la cuenta no está activada, se rechaza con un error **distinto** al de credenciales inválidas (no se oculta esta condición — ver tabla de respuestas). |
| Transporte y materialización | HTTP JSON, `POST /v1/tokens/authentication`. Ver contrato abajo. |
| Scope concreto | Constante `data.ScopeAuthentication = "authentication"` en `internal/data/tokens.go`, junto a `ScopeActivation`. |

---

## Contrato HTTP

### Request

```
POST /v1/tokens/authentication
Content-Type: application/json

{
  "email": "alice@example.com",
  "password": "pa55word"
}
```

### Respuestas

| Caso | Código | Body (forma) | Regla que satisface |
|---|---|---|---|
| Éxito | `201 Created` | `{"authentication_token": {"token": "<plaintext>", "expiry": "<RFC3339>"}}` | R4 (expiry visible), R5 (secreto solo aquí, una vez) |
| Formato inválido (email vacío/mal formado, password fuera de longitud) | `422 Unprocessable Entity` | `{"error": {"email": "...", "password": "..."}}` (formato ya usado por `failedValidationResponse`) | R3 — se valida antes de tocar el almacén |
| Email no existe **o** password no coincide | `401 Unauthorized` | `{"error": "invalid authentication credentials"}` — **mismo mensaje y código en ambos casos** | R2 |
| Email y password correctos, cuenta no activada | `403 Forbidden` | `{"error": "your user account must be activated to authenticate"}` (reutiliza el mismo caso de uso que un futuro `inactiveAccountResponse`) | Precondición de cuenta (tabla arriba) — no es R2 porque aquí la cuenta sí existe y la credencial sí es correcta; no hay ambigüedad que proteger |
| Fallo de infraestructura (DB caída, etc.) | `500 Internal Server Error` | `{"error": "the server encountered a problem..."}` (`serverErrorResponse` ya existente) | Escenario "fallo del sistema" del padre — distinguible de 401/403 |

**Nota sobre `403` vs. R2:** el padre exige indistinguibilidad solo entre "no existe" y "credencial incorrecta". El estado de activación es una tercera condición sobre una cuenta ya autenticada con éxito (la credencial fue correcta) — filtrarla no es una fuga de enumeración de cuentas, es información que el propio dueño de la cuenta necesita para saber qué hacer a continuación (revisar su correo de activación).

---

## Cambios de código

### `internal/data/tokens.go`

```go
const (
    ScopeActivation     = "activation"
    ScopeAuthentication = "authentication"
)

type Token struct {
    Plaintext string    `json:"token"`
    Hash      []byte    `json:"-"`
    UserID    int64     `json:"-"`
    Expiry    time.Time `json:"expiry"`
    Scope     string    `json:"-"`
}
```

No se toca `generateToken`, `New`, `Insert` ni `DeleteAllForUser` — el mecanismo ya es genérico por `scope` (cumple R6: el scope acota el propósito del token, y esto ya estaba resuelto por el diseño existente).

### `cmd/api/tokens.go` (archivo nuevo)

Handler `createAuthenticationTokenHandler`, mismo patrón que `registerUserHandler`/`activateUserHandler` en `cmd/api/users.go`:

1. `readJSON` → `input struct { Email, Password string }`.
2. `data.ValidateEmail` + `data.ValidatePasswordPlaintext` → si falla, `failedValidationResponse` (422). **Esto ocurre antes del paso 3** (R3).
3. `app.models.Usr.GetByEmail(input.Email)`:
   - `data.ErrRecordNotFound` → `invalidCredentialsResponse` (401).
   - otro error → `serverErrorResponse` (500).
4. `user.Password.Matches(input.Password)`:
   - error no nil → `serverErrorResponse` (500).
   - `false` → `invalidCredentialsResponse` (401) — **mismo helper que el paso 3**, para que R2 se cumpla por construcción (un solo punto de respuesta para ambos casos, no dos mensajes que puedan divergir).
5. `!user.Activated` → `inactiveAccountResponse` (403).
6. `app.models.Tkn.New(user.ID, 24*time.Hour, data.ScopeAuthentication)` → error → `serverErrorResponse` (500).
7. `writeJSON(201, envelope{"authentication_token": token}, nil)`.

### `cmd/api/errors.go`

Agregar dos helpers nuevos (mismo estilo que los existentes `failedValidationResponse`, `editConflictResponse`):

```go
func (app *application) invalidCredentialsResponse(w http.ResponseWriter, r *http.Request) {
    app.errorResponse(w, r, http.StatusUnauthorized, "invalid authentication credentials")
}

func (app *application) inactiveAccountResponse(w http.ResponseWriter, r *http.Request) {
    app.errorResponse(w, r, http.StatusForbidden, "your user account must be activated to authenticate")
}
```

Verificado contra `cmd/api/errors.go`: ninguno de los dos existe aún; el patrón (mensaje fijo + `app.errorResponse(w, r, status, message)`) es idéntico al de `editConflictResponse` y `rateLimitExceededResponse`, así que ambos helpers van al final de ese archivo sin fricción de estilo.

### `cmd/api/routes.go`

```go
router.HandlerFunc(http.MethodPost, "/v1/tokens/authentication", app.createAuthenticationTokenHandler)
```

---

## Trazabilidad con el padre

- R1 → paso 4, `bcrypt` vía `Matches`.
- R2 → pasos 3 y 4 comparten el mismo helper de respuesta (`invalidCredentialsResponse`).
- R3 → paso 2 antes que paso 3.
- R4 → `Expiry` siempre seteado en `generateToken` (ya existente), expuesto en el JSON de respuesta.
- R5 → `Plaintext` solo se serializa en la respuesta de creación; `Hash` tiene `json:"-"`; en DB solo se persiste `Hash`.
- R6 → `Scope: "authentication"` fijo, distinto de `"activation"`.

---

## Fuera de alcance de este hijo

- Middleware de autenticación que **use** este token en requests posteriores (lectura de `Authorization: Bearer`, resolución a usuario) — es un hijo separado, consumidor de este.
- Rate limiting específico anti brute-force más allá del `rateLimitWithIP` global ya existente.
- Endpoint de logout / revocación explícita (ya existe `DeleteAllForUser`, pero no está expuesto por HTTP).
