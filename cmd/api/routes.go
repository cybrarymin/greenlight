package api

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (app *application) routes() http.Handler {
	router := httprouter.New()

	router.NotFound = http.HandlerFunc(app.notFoundResponse)
	router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowedResponse)
	router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.otelHandler(app.JWTAuth(app.healthcheckHandler)))

	// Movies Handlers
	router.HandlerFunc(http.MethodPost, "/v1/movies", app.otelHandler(app.Auth(app.requireActivatedUser(app.requirePermission("movies:write", app.createMovieHandler)))))
	router.HandlerFunc(http.MethodGet, "/v1/movies", app.otelHandler(app.Auth(app.requireActivatedUser(app.requirePermission("movies:read", app.listMovieHandler)))))
	router.HandlerFunc(http.MethodGet, "/v1/movies/:id", app.otelHandler(app.Auth(app.requireActivatedUser(app.requirePermission("movies:read", app.showMovieHandler)))))
	router.HandlerFunc(http.MethodPatch, "/v1/movies/:id", app.otelHandler(app.Auth(app.requireActivatedUser(app.requirePermission("movies:write", app.updateMovieHandler)))))
	router.HandlerFunc(http.MethodDelete, "/v1/movies/:id", app.otelHandler(app.Auth(app.requireActivatedUser(app.requirePermission("movies:write", app.deleteMovieHandler)))))

	// User Handlers
	router.HandlerFunc(http.MethodPost, "/v1/users", app.otelHandler(app.Auth(app.registerUserHandler)))
	router.HandlerFunc(http.MethodGet, "/v1/users", app.otelHandler(app.Auth(app.ListUserHandler)))
	router.HandlerFunc(http.MethodDelete, "/v1/users/:id", app.otelHandler(app.Auth(app.DeleteUserHandler)))

	// token activation Handlers
	router.HandlerFunc(http.MethodPut, "/v1/users/:id/activate", app.otelHandler(app.Auth(app.userActivationHandler)))

	// authentication token Handlers
	// createBearerTokenHandler has basic authentication within itself
	router.HandlerFunc(http.MethodPost, "/v1/tokens/auth", app.otelHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.createBearerTokenHandler(w, r)
	})))

	router.HandlerFunc(http.MethodPost, "/v1/tokens/jwt", app.otelHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.createJWTTokenHandler(w, r)
	})))

	// application metrics Handlers
	router.Handler(http.MethodGet, "/metrics", promhttp.Handler())

	return app.PanicRecovery(app.enableCORS(app.RateLimit(router)))
}
