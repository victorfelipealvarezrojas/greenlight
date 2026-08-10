# Spec padre — Emisión de token de autenticación

> **Nivel:** padre (agnóstico, portable). Describe *qué debe cumplirse siempre*,
> en cualquier lenguaje y transporte. No contiene HTTP, ni nombres de lenguaje,
> ni valores concretos. Cada implementación cuelga de aquí como un **spec hijo**
> que concreta lo que este documento deja abierto.
>
> Regla de herencia: un hijo **concreta** o **extiende**, nunca **contradice**.
> Si un hijo necesita romper una regla de abajo, o el padre estaba mal destilado
> o es una variante que merece su propia rama.

---

## Principio

Un cliente intercambia sus credenciales, una sola vez, por un **token** de vida
acotada que lo identifica en interacciones posteriores. La verificación costosa
de la credencial ocurre únicamente en el momento de la emisión; a partir de ahí,
el token la reemplaza.

---

## Comportamiento observable

**Escenario — emisión exitosa**
Dado un identificador de usuario y una credencial correctos y bien formados,
cuando se solicita un token,
entonces el sistema emite un token nuevo asociado a ese usuario, con expiración,
y lo entrega una sola vez.

**Escenario — credenciales inválidas**
Dado un identificador que no corresponde a ningún usuario, o una credencial que
no coincide,
cuando se solicita el token,
entonces se rechaza con una respuesta de credenciales inválidas **idéntica en
ambos casos**, sin revelar cuál de los dos falló.

**Escenario — formato inválido**
Dado un identificador o una credencial mal formados,
cuando se solicita el token,
entonces se rechaza por formato **antes** de consultar el almacén de usuarios.

**Escenario — fallo del sistema**
Dado un fallo al recuperar el usuario distinto de "no existe",
cuando se solicita el token,
entonces se responde con un error de sistema, **distinguible** del rechazo por
credenciales.

---

## Reglas invariantes

- **R1** — La credencial se verifica con una comparación costosa (hash lento) y
  solo en la emisión, nunca en usos posteriores del token.
- **R2** — Usuario inexistente y credencial incorrecta son indistinguibles para
  el cliente.
- **R3** — La validación de formato precede a cualquier acceso al almacén de
  usuarios.
- **R4** — Todo token emitido tiene expiración.
- **R5** — El secreto del token se entrega una sola vez, en la emisión. El
  sistema nunca persiste el secreto en claro; si lo persiste, guarda solo una
  forma no reversible (hash).
- **R6** — El token acota su alcance a un propósito (scope); no es un permiso en
  blanco.

---

## Decisiones que el padre deja abiertas

Cada spec hijo debe cerrar estas, y al cerrarlas no puede violar las reglas de
arriba:

- **Persistencia** — stateful (token opaco cuyo estado vive en un almacén,
  revocable por borrado, con costo de consulta por uso) vs. stateless
  (autocontenido y firmado, sin consulta, con revocación como problema abierto).
- **Valor de expiración** — el concreto (R4 solo exige que exista).
- **Forma de la credencial** — identificador + secreto; qué es el identificador
  (correo, nombre de usuario, otro) es del hijo.
- **Precondiciones de cuenta** — si se exige algún estado del usuario (por
  ejemplo, cuenta habilitada o verificada) antes de emitir.
- **Transporte y materialización** — protocolo, dónde viaja el token, códigos y
  formas de respuesta para éxito, credencial inválida, formato y fallo de
  sistema.
- **Nombre y valor del scope** concreto.

---

## Qué no pertenece a este documento

Nombres de lenguaje o framework, rutas o verbos de transporte, estructuras de
datos concretas, códigos de estado, valores numéricos. Todo eso vive en los spec
hijos. Si algo de eso aparece aquí, este documento dejó de ser padre.
