package main

import (
	"expvar"
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

	// improvement add metrics:read permission
	router.Handler(http.MethodGet, "/v1/metrics", expvar.Handler())

	router.HandlerFunc(http.MethodPost, "/v1/movies", app.requirePermission(data.PermissionMoviesWrite, app.createMovieHandler))
	router.HandlerFunc(http.MethodGet, "/v1/movies", app.requirePermission(data.PermissionMoviesRead, app.listMovieHandler))
	router.HandlerFunc(http.MethodGet, "/v1/movies/:id", app.requirePermission(data.PermissionMoviesRead, app.showMovieHandler))
	router.HandlerFunc(http.MethodPatch, "/v1/movies/:id", app.requirePermission(data.PermissionMoviesWrite, app.updateMovieHandler))
	router.HandlerFunc(http.MethodDelete, "/v1/movies/:id", app.requirePermission(data.PermissionMoviesWrite, app.deleteMovieHandler))

	router.HandlerFunc(http.MethodPost, "/v1/accounts/register", app.registerUserHandler)
	router.HandlerFunc(http.MethodPut, "/v1/accounts/activate", app.activateUserHandler)
	router.HandlerFunc(http.MethodPut, "/v1/accounts/password-reset", app.updateUserPasswordHandler)

	router.HandlerFunc(http.MethodPost, "/v1/tokens/accounts/login", app.createAuthenticationTokenHandler)
	router.HandlerFunc(http.MethodPost, "/v1/tokens/accounts/forgot-password", app.createPasswordResetTokenHandler)
	router.HandlerFunc(http.MethodPost, "/v1/tokens/accounts/resend-activation-token", app.createActivationTokenHandler)

	// apply middleware to all routes
	// flow:- metrics -> recoverPanic -> enableCORS -> rateLimit -> authenticate -> requireActivatedUser
	return app.metrics(app.recoverPanic(app.enableCORS(app.rateLimit(app.authenticate(router)))))
}
