# Spec: Módulo de Users

**Versión:** 1.0
**Estado:** Aprobado para implementación
**Owner:** Backend
**Package:** `internal/data`

---

## 1. Contexto y objetivo

El sistema requiere un módulo de gestión de usuarios que soporte los flujos de registro, autenticación, y recuperación de credenciales. Este spec define el contrato del tipo `User`, el manejo seguro de contraseñas, la validación de entrada, y la persistencia en PostgreSQL.

El módulo debe ser consumible por handlers HTTP sin exponer detalles de implementación de hashing ni de la base de datos, y debe garantizar que ningún `User` con estado inválido pueda persistir.

## 2. Alcance

**Incluido en esta spec:**

- Tipo `User` y su tipo interno `password`.
- Funciones de validación reutilizables por campo y a nivel de agregado.
- Interfaz de persistencia `UserModel` con operaciones CRUD y lookup por email.
- Errores de dominio exportados.

**Fuera de alcance:**

- Handlers HTTP.
- Envío de emails de activación.
- Sistema de tokens (registro, autenticación, reset). Se especifica por separado.
- Rate limiting.
- Middleware de autenticación.

## 3. Dependencias

- `golang.org/x/crypto/bcrypt` — hashing de contraseñas.
- `database/sql` con driver `pq` — persistencia PostgreSQL.
- Paquete interno `internal/validator` — helpers `Check`, `Matches`, `EmailRX`.
- Go 1.22+.

## 4. Tipo `User`

### 4.1 Campos

| Campo | Tipo | Serialización JSON | Notas |
|---|---|---|---|
| `ID` | `int64` | `id` | Generado por la DB. |
| `CreatedAt` | `time.Time` | `created_at` | Generado por la DB. |
| `Name` | `string` | `name` | Requerido. |
| `Email` | `string` | `email` | Único en la DB, case-insensitive vía `citext`. |
| `Password` | `password` | **omitido** (`json:"-"`) | Nunca serializar. |
| `Activated` | `bool` | `activated` | Default `false`. |
| `Version` | `int` | **omitido** (`json:"-"`) | Optimistic locking. |

### 4.2 Invariantes del agregado

- `Email` debe estar presente y con formato válido en cualquier `User` que llegue a persistencia.
- `Name` debe estar presente y no exceder 500 bytes.
- `Password.hash` debe estar poblado antes de invocar cualquier operación de persistencia. Un hash `nil` en persistencia es un bug de programación, no un error de datos.

## 5. Tipo `password`

### 5.1 Campos

- `plaintext *string` — puntero para distinguir tres estados:
    - `nil` → no seteado (usuario cargado desde DB, no viene del cliente).
    - `&""` → seteado explícitamente vacío (debe fallar validación).
    - `&"valor"` → seteado con contenido.
- `hash []byte` — hash bcrypt del plaintext.

Ambos campos son **no exportados**. La interacción con el tipo ocurre exclusivamente vía los métodos `Set` y `Matches`.

### 5.2 Método `Set(plaintextPassword string) error`

- Genera el hash con bcrypt cost **12**.
- Poblará `plaintext` con puntero al valor recibido y `hash` con el resultado.
- Devuelve error solo si bcrypt falla (memoria, cost fuera de rango).

### 5.3 Método `Matches(plaintextPassword string) (bool, error)`

- Compara `plaintextPassword` contra `p.hash` usando `bcrypt.CompareHashAndPassword`.
- Retorno:
    - `(true, nil)` si coinciden.
    - `(false, nil)` si no coinciden (error `bcrypt.ErrMismatchedHashAndPassword` clasificado como resultado, no como fallo).
    - `(false, err)` para cualquier otro error de bcrypt.

## 6. Validación

Las funciones de validación operan sobre un `*validator.Validator` compartido y acumulan errores; no retornan valor.

### 6.1 `ValidateEmail(v, email string)`

- `email != ""` con mensaje `"must be provided"`.
- `validator.Matches(email, validator.EmailRX)` con mensaje `"must be a valid email address"`.

### 6.2 `ValidatePasswordPlaintext(v, password string)`

- `password != ""` con mensaje `"must be provided"`.
- `len(password) >= 8` con mensaje `"must be at least 8 bytes long"`.
- `len(password) <= 72` con mensaje `"must not be more than 72 bytes long"`.

El límite superior de 72 es una restricción real de bcrypt: solo hashea los primeros 72 bytes del input. Rechazar en validación evita que dos passwords distintos con prefijo común coincidan silenciosamente.

### 6.3 `ValidateUser(v, user *User)`

Compone las anteriores más reglas propias del agregado:

- `user.Name != ""` con mensaje `"must be provided"`.
- `len(user.Name) <= 500` con mensaje `"must not be more than 500 bytes long"`.
- Invoca `ValidateEmail(v, user.Email)`.
- Si `user.Password.plaintext != nil`, invoca `ValidatePasswordPlaintext(v, *user.Password.plaintext)`. La condición es intencional: al leer un user desde DB, el plaintext no existe y esa rama debe saltarse.
- Si `user.Password.hash == nil`, **panic** con mensaje `"missing password hash for user"`. Es un bug del llamador (invocó validación antes de hashear); no es un error reportable al cliente.

### 6.4 Regla de composición

`ValidateEmail` y `ValidatePasswordPlaintext` deben ser **invocables independientemente** de `ValidateUser`. Son consumidas por endpoints donde no existe un `*User` completo (login, solicitud de reset, cambio de password). Cualquier cambio en las reglas debe ocurrir en un único lugar y propagarse por composición.

## 7. Errores de dominio

Los siguientes errores exportados deben estar disponibles en el paquete y ser devueltos por `UserModel` en los casos especificados. Se comparan con `errors.Is` desde los handlers.

- `ErrDuplicateEmail = errors.New("duplicate email")` — devuelto por `Insert` cuando la constraint única de email es violada.
- `ErrRecordNotFound` — ya existente en el paquete; devuelto por `GetByEmail` y `Get` cuando el registro no existe.
- `ErrEditConflict` — ya existente; devuelto por `Update` cuando el `Version` no coincide (conflicto optimista).

Los errores crudos del driver (`pq.Error`) **no deben propagarse** fuera de `UserModel`. La traducción ocurre dentro del método correspondiente.

## 8. Contrato de `UserModel`

Struct con dependencia `DB *sql.DB`. Todos los métodos aceptan `context.Context` como primer parámetro y respetan cancelación con timeout de 3 segundos vía `context.WithTimeout`.

### 8.1 `Insert(ctx context.Context, user *User) error`

- Inserta `name`, `email`, `password_hash`, `activated`.
- Retorno de DB: `id`, `created_at`, `version` escritos en el `*User`.
- Errores esperados:
    - Constraint `users_email_key` violada → devolver `ErrDuplicateEmail`.
    - Cualquier otro error del driver → propagar tal cual.

### 8.2 `GetByEmail(ctx context.Context, email string) (*User, error)`

- Lookup por columna `email` (comparación case-insensitive por tipo `citext` de la columna).
- Populariza todos los campos incluido `password.hash`. `password.plaintext` queda `nil`.
- Errores:
    - `sql.ErrNoRows` → devolver `ErrRecordNotFound`.
    - Otros → propagar.

### 8.3 `Update(ctx context.Context, user *User) error`

- Update de `name`, `email`, `password_hash`, `activated` filtrando por `id` **y** `version`.
- Incrementa `version` en la sentencia.
- Errores:
    - Cero filas afectadas → devolver `ErrEditConflict`.
    - Constraint de email → devolver `ErrDuplicateEmail`.
    - Otros → propagar.

## 9. Restricciones no funcionales

### 9.1 Seguridad

- El campo `Password` del tipo `User` **nunca** debe aparecer en output JSON. Verificado por el tag `json:"-"`.
- El plaintext no debe loggearse en ningún método. Los logs de error de bcrypt deben omitir el input.
- El hash bcrypt no debe exponerse en respuestas HTTP ni en logs de aplicación bajo ninguna circunstancia.
- Cost bcrypt **12** es no negociable sin revisión de threat model.

### 9.2 Serialización

- `Version` marcado `json:"-"` para evitar filtrado del mecanismo de locking al cliente.
- `CreatedAt` se serializa por default de `time.Time`. Formato RFC3339 aceptable.

### 9.3 Performance

- Todos los métodos de DB usan `QueryRowContext` / `ExecContext` con timeout de 3s.
- El costo bcrypt 12 implica ~250ms por hash en hardware típico. `Insert` y `Update` con nueva password deben tolerar esa latencia sin bloquear otros requests (Go maneja la concurrencia por goroutine; no hay contención global salvo la del pool de conexiones DB).

## 10. Criterios de aceptación

### 10.1 Comportamiento

- [ ] Un `User` con email inválido no puede pasar `ValidateUser` sin acumular error.
- [ ] `password.Set` seguido de `password.Matches` con el mismo plaintext devuelve `(true, nil)`.
- [ ] `password.Matches` con plaintext incorrecto devuelve `(false, nil)`, no error.
- [ ] `Insert` con email duplicado devuelve exactamente `ErrDuplicateEmail`.
- [ ] `GetByEmail` con email inexistente devuelve exactamente `ErrRecordNotFound`.
- [ ] `Update` con `Version` desactualizado devuelve exactamente `ErrEditConflict`.
- [ ] Serializar un `User` a JSON no incluye `password` ni `version` en el output.
- [ ] `ValidateUser` con `password.hash == nil` produce panic.

### 10.2 Aislamiento

- [ ] Los métodos de `UserModel` no devuelven errores tipo `*pq.Error` al llamador.
- [ ] `ValidateEmail` y `ValidatePasswordPlaintext` son invocables sin construir un `*User`.

## 11. Tests mínimos requeridos

Se espera cobertura en los siguientes casos, como mínimo:

**Sobre `password`:**
- `Set` + `Matches` roundtrip exitoso.
- `Matches` con password incorrecto retorna `(false, nil)`.
- `Set` con string vacío no retorna error (la validación es responsabilidad de otra capa).

**Sobre validación:**
- `ValidateEmail` con email vacío, malformado, y válido.
- `ValidatePasswordPlaintext` en los tres bordes: vacío, 7 chars, 73 chars, 8 chars, 72 chars.
- `ValidateUser` con hash `nil` produce panic (usar `defer recover` en test).
- `ValidateUser` con `plaintext == nil` no falla por password (caso de user leído de DB).

**Sobre `UserModel`:**
- Requieren DB de test. `Insert` de email duplicado retorna `ErrDuplicateEmail` exacto (comparar con `errors.Is`).
- `GetByEmail` inexistente retorna `ErrRecordNotFound` exacto.
- `Update` con version stale retorna `ErrEditConflict` exacto.

## 12. Migración DB requerida

La implementación asume la existencia de la tabla `users` con el siguiente esquema mínimo. La migración es entregable separado.

```sql
CREATE TABLE IF NOT EXISTS users (
    id bigserial PRIMARY KEY,
    created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
    name text NOT NULL,
    email citext UNIQUE NOT NULL,
    password_hash bytea NOT NULL,
    activated bool NOT NULL,
    version integer NOT NULL DEFAULT 1
);
```

Requiere extensión `citext` habilitada.

## 13. Decisiones pendientes / fuera de esta spec

- Política de activación (email con token vs auto-activación).
- Política de expiración de sesión.
- Auditoría de intentos fallidos de login.

---

**Fin del documento.**