package api

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func (app *application) routes() http.Handler {
	router := httprouter.New()

	router.NotFound = http.HandlerFunc(app.notFoundResponse)
	router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowedResponse)
	router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthcheckHandler)

	// Movies Handlers
	router.HandlerFunc(http.MethodPost, "/v1/movies", app.BearerAuth(app.createMovieHandler))
	router.HandlerFunc(http.MethodGet, "/v1/movies", app.BearerAuth(app.listMovieHandler))
	router.HandlerFunc(http.MethodGet, "/v1/movies/:id", app.BearerAuth(app.showMovieHandler))
	router.HandlerFunc(http.MethodPatch, "/v1/movies/:id", app.BearerAuth(app.updateMovieHandler))
	router.HandlerFunc(http.MethodDelete, "/v1/movies/:id", app.BearerAuth(app.deleteMovieHandler))

	// User Handlers
	router.HandlerFunc(http.MethodPost, "/v1/users", app.BearerAuth(app.registerUserHandler))
	router.HandlerFunc(http.MethodGet, "/v1/users", app.BearerAuth(app.ListUserHandler))
	router.HandlerFunc(http.MethodDelete, "/v1/users/:id", app.BearerAuth(app.DeleteUserHandler))

	// token activation Handlers
	router.HandlerFunc(http.MethodPut, "/v1/users/:id/activate", app.BearerAuth(app.userActivationHandler))

	// authentication token Handlers
	// createBearerTokenHandler has basic authentication within itself
	router.HandlerFunc(http.MethodPost, "/v1/tokens/auth", app.createBearerTokenHandler)

	return app.PanicRecovery(app.RateLimit(router))
}
