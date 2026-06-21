package main

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

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

// ES NECESARIO PARA QUE EL SERVIDOR RESPONDA CON UN ERROR 500 EN CASO DE UN PANIC, DE LO CONTRARIO EL SERVIDOR SE CAERÍA SILENCIOSAMENTE
// SIN RESPONDER AL CLIENTE, LO QUE ES MUY PROBLEMÁTICO PARA LOS USUARIOS Y LOS SISTEMAS DE MONITOREO. EL SERVIDOR DEBE RESPONDER CON UN
//  ERROR 500 PARA INDICAR QUE HA OCURRIDO UN PROBLEMA INTERNO, PERMITIENDO ASÍ A LOS USUARIOS Y LOS SISTEMAS DE MONITOREO DETECTAR Y
// RESPONDER A LA SITUACIÓN DE MANERA ADECUADA.

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

func (app *application) rateLimitWithIP(next http.Handler) http.Handler {
	type client struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}

	var (
		mu      sync.Mutex
		clients = make(map[string]*client)
	)

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
		if app.config.limiter.enabled {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				app.serverErrorResponse(w, r, err)
				return
			}
			mu.Lock()
			if _, found := clients[ip]; !found {
				clients[ip] = &client{
					limiter: rate.NewLimiter(rate.Limit(app.config.limiter.rps), app.config.limiter.burst),
				}
			}
			clients[ip].lastSeen = time.Now()
			if !clients[ip].limiter.Allow() {
				mu.Unlock()
				app.rateLimitExceededResponse(w, r)
				return
			}
			mu.Unlock()
		}
		next.ServeHTTP(w, r)
	})

}

// Deprecated: usa rateLimit, que aplica el límite por IP.
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
