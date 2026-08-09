package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/valvarez/greenlight/internal/data"
	"github.com/valvarez/greenlight/internal/validator"
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
				app.serverErrorResponse(w, r, fmt.Errorf("error panic %s", err))
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

func (app *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Agregue el encabezado "Vary: Authorization" a la respuesta. Esto indica a cualquier
		// almacena en caché que la respuesta puede variar según el valor de la Autorización
		// encabezado en la solicitud.
		w.Header().Add("Vary", "Authorization")

		// Recupera el valor del encabezado de Autorización de la solicitud. esto será
		// devuelve la cadena vacía "" si no se encuentra dicho encabezado.
		authorizationHeader := r.Header.Get("Authorization")

		// Si no se encuentra ningún encabezado de Autorización, use el asistente contextSetUser()
		// que acabamos de hacer para agregar AnonymousUser al contexto de solicitud.
		// entonces llama al siguiente controlador de la cadena y regresa sin ejecutar ninguno de los
		// código a continuación.
		if authorizationHeader == "" {
			r = app.contextSetUser(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		// De lo contrario, esperamos que el valor del encabezado de Autorización tenga el formato
		// Bearer <token>. Intentamos dividir esto en sus partes constituyentes, y si el
		// el encabezado no tiene el formato esperado, devolvemos una respuesta 401 no autorizada
		// usando el asistente invalidAuthenticationTokenResponse() (que crearemos
		// en un momento).
		headerParts := strings.Split(authorizationHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}

		token := headerParts[1]

		v := validator.New()
		if data.ValidateTokenPlaintext(v, token); !v.Valid() {
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}

		// Retrieve the details of the user associated with the authentication token,
		// again calling the invalidAuthenticationTokenResponse() helper if no
		// matching record was found. IMPORTANT: Notice that we are using
		// ScopeAuthentication as the first parameter here.
		user, err := app.models.Usr.GetForToken(data.ScopeAuthentication, token)
		if err != nil {
			switch {
			case errors.Is(err, data.ErrRecordNotFound):
				app.invalidAuthenticationTokenResponse(w, r)
			default:
				app.serverErrorResponse(w, r, err)
			}
			return
		}

		r = app.contextSetUser(r, user)
		next.ServeHTTP(w, r)
	})
}

// Crea un nuevo middleware requireAuthenticatedUser() para comprobar que un usuario no está
// anónimo.
func (app *application) requireAuthenticatedUser(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)
		if user.IsAnonymous() {
			app.authenticationRequiredResponse(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (app *application) requireActivatedUser(next http.HandlerFunc) http.HandlerFunc {
	// En lugar de devolver este http.HandlerFunc, lo asignamos a la variable fn.
	fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)
		if !user.Activated {
			app.inactiveAccountResponse(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
	// Envuelve fn con el middleware requireAuthenticatedUser() antes de devolverlo.
	return app.requireAuthenticatedUser(fn)
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

// el primer parámetro para la función de middleware es el código de permiso que
// requerimos que el usuario tenga.
func (app *application) requirePermission(code string, next http.HandlerFunc) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)
		permissions, err := app.models.Permissions.GetAllForUser(user.ID)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		if !permissions.Include(code) {
			app.notPermittedResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	}
	// Envuelva esto con el middleware requireActivatedUser() antes de devolverlo.
	return app.requireActivatedUser(fn)
}

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
