package main

import (
	"greenlight/internal/data"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func (app *application) routes() http.Handler {
	router := httprouter.New()

	// without this server would return plain text 404 response
	router.NotFound = http.HandlerFunc(app.notFoundResponse)
	// without this server would return plain text 405 response
	router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowedResponse)

	router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthCheckHandler)

	router.HandlerFunc(http.MethodPost, "/v1/movies", app.requirePermission(data.PermissionMoviesWrite, app.createMovieHandler))
	router.HandlerFunc(http.MethodGet, "/v1/movies", app.requirePermission(data.PermissionMoviesRead, app.listMovieHandler))
	router.HandlerFunc(http.MethodGet, "/v1/movies/:id", app.requirePermission(data.PermissionMoviesRead, app.showMovieHandler))
	router.HandlerFunc(http.MethodPatch, "/v1/movies/:id", app.requirePermission(data.PermissionMoviesWrite, app.updateMovieHandler))
	router.HandlerFunc(http.MethodDelete, "/v1/movies/:id", app.requirePermission(data.PermissionMoviesWrite, app.deleteMovieHandler))

	router.HandlerFunc(http.MethodPost, "/v1/users", app.registerUserHandler)
	router.HandlerFunc(http.MethodPut, "/v1/users/activate", app.activateUserHandler)

	router.HandlerFunc(http.MethodPost, "/v1/tokens/authentication", app.createAuthenticationTokenHandler)

	// apply middleware to all routes
	// flow recoverPanic -> rateLimit -> authenticate -> requireActivatedUser
	return app.recoverPanic(app.rateLimit(app.authenticate(router)))
}
