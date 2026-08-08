package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func (app *application) routes() http.Handler {
	router := httprouter.New()

	// Convierte el asistente notFoundResponse() en http.Handler usando el
	// adaptador http.HandlerFunc() y luego se configura como el controlador de errores personalizado para 404
	// Respuestas no encontradas.
	router.NotFound = http.HandlerFunc(app.notFoundResponse)

	// Del mismo modo, convierte el asistente métodoNotAllowedResponse() en http.Handler y configura
	// como controlador de errores personalizado para las respuestas 405 Método no permitido.
	router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowedResponse)

	router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthcheckHandler)

	router.HandlerFunc(http.MethodGet, "/v1/movies", app.requirePermission("movies:read", app.listMoviesHandler))
	router.HandlerFunc(http.MethodPost, "/v1/movies", app.requirePermission("movies:write", app.createMovieHandler))
	router.HandlerFunc(http.MethodGet, "/v1/movies/:id", app.requirePermission("movies:read", app.showMovieHandler))
	router.HandlerFunc(http.MethodPatch, "/v1/movies/:id", app.requirePermission("movies:write", app.updateMovieHandler))
	router.HandlerFunc(http.MethodDelete, "/v1/movies/:id", app.requirePermission("movies:write", app.deleteMovieHandler))

	router.HandlerFunc(http.MethodPost, "/v1/users", app.registerUserHandler)
	router.HandlerFunc(http.MethodPut, "/v1/users/activated", app.activateUserHandler)

	router.HandlerFunc(http.MethodPost, "/v1/tokens/authentication", app.createAuthenticationTokenHandler)

	// recoverPanic no se gatilla en la salida — envuelve toda la ejecución.
	// rateLimit se gatilla antes de recoverPanic para recuperar de cualquier panic que pueda ocurrir dentro del middleware de limitación de velocidad.
	return app.recoverPanic(app.rateLimit(app.authenticate(router)))
}
