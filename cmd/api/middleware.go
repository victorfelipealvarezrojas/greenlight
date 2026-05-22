package main

import (
	"fmt"
	"net/http"
)

// recoverPanic es un middleware que envuelve toda la cadena de ejecución del request.
// Está activo desde que el cliente hace la petición hasta que el handler responde.
//
// Flujo normal:
//
//	request → [recoverPanic abre defer] → router → handler → response
//
// Flujo con panic:
//
//	request → [recoverPanic abre defer] → router → handler
//	→ Go desenrolla el stack → defer se ejecuta → recover() atrapa el panic
//	→ responde 500 al cliente de forma ordenada
//
// IMPORTANTE: recover() es exclusivo para panics, no atrapa errores normales de Go.
// Los errores normales (err != nil) fluyen como valores de retorno y son
// responsabilidad de cada handler. recover() actúa como red de seguridad
// para bugs inesperados — nil pointer dereference, index out of range, etc.
// que de otro modo matarían la goroutine silenciosamente sin responder al cliente.
//
// El header "Connection: close" le indica al servidor HTTP de Go que cierre
// la conexión de forma ordenada después de enviar la respuesta, en lugar
// de dejarla colgada sin explicación.
//
// Se registra en routes() envolviendo el router completo:
//
//	return app.recoverPanic(router)
//
// lo que significa que aplica a todas las rutas sin excepción.
func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a deferred function (which will always be run in the event of a panic
		// as Go unwinds the stack).
		defer func() {
			// Use the builtin recover function to check if there has been a panic or
			// not.
			if err := recover(); err != nil {
				// If there was a panic, set a "Connection: close" header on the
				// response. This acts as a trigger to make Go's HTTP server
				// automatically close the current connection after a response has been
				// sent.
				w.Header().Set("Connection", "close")
				// The value returned by recover() has the type any, so we use
				// fmt.Errorf() to normalize it into an error and call our
				// serverErrorResponse() helper. In turn, this will log the error using
				// our custom Logger type at the ERROR level and send the client a 500
				// Internal Server Error response.
				app.serverErrorResponse(w, r, fmt.Errorf("%s", err))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
