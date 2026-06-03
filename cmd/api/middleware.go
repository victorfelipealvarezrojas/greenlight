package main

import (
	"fmt"
	"net/http"

	"golang.org/x/time/rate" // Importa el paquete rate para implementar limitación de velocidad
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

func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.Header().Set("Connection", "close")
				app.serverErrorResponse(w, r, fmt.Errorf("%s", err))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (app *application) rateLimit(next http.Handler) http.Handler {
	// Inicializa un nuevo limitador de velocidad que permite un promedio de 2 solicitudes por segundo,
	// con un máximo de 4 solicitudes en una sola 'ráfaga'.
	limiter := rate.NewLimiter(2, 4)

	// La función que devolvemos es un clousure, que 'cierra' el limitador
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Llame a limiter.Allow() para ver si la solicitud está permitida y, si no,
		// luego llamamos al asistente rateLimitExceededResponse() para devolver 429
		if !limiter.Allow() {
			app.rateLimitExceededResponse(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})

}
