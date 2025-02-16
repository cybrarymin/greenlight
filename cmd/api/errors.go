package api

import (
	"fmt"
	"net/http"
)

type Envelope map[string]interface{}

// logError is the method we use to log the errors happens on the server side for the application.
func (app *application) logError(err error) {
	app.log.Error().Err(err).Send()
}

// errorResponse is the method we use to send a json formatted error to the client in case of any error
func (app *application) errorResponse(w http.ResponseWriter, r *http.Request, status int, message interface{}) {
	e := envelope{
		"error": message,
	}
	err := app.writeJson(w, status, e, nil)

	if err != nil {
		app.logError(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// serverErrorResponse uses the two other methods to log the details of the error and send internal server error to the client
func (app *application) serverErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.logError(err)
	message := "the server encountered an error to process the request"
	app.errorResponse(w, r, http.StatusInternalServerError, message)
}

// notFoundResponse method will be used to send notFound 404 status error json response to the client
func (app *application) notFoundResponse(w http.ResponseWriter, r *http.Request) {
	message := "the requested resource couldn't be found"
	app.errorResponse(w, r, http.StatusNotFound, message)
}

func (app *application) badRequestResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.errorResponse(w, r, http.StatusBadRequest, err.Error())
}

// methodNotAllowed method will be used to send notFound 404 status error json response to the client
func (app *application) methodNotAllowedResponse(w http.ResponseWriter, r *http.Request) {
	message := fmt.Sprintf("the %s method is not supported for this resource", r.Method)
	app.errorResponse(w, r, http.StatusMethodNotAllowed, message)
}

func (app *application) failedValidationResponse(w http.ResponseWriter, r *http.Request, errors map[string]string) {
	app.errorResponse(w, r, http.StatusUnprocessableEntity, errors)
}

func (app *application) editConflictResponse(w http.ResponseWriter, r *http.Request) {
	message := "unable to update the record due to an edit conflict, please try again"
	app.errorResponse(w, r, http.StatusConflict, message)
}

func (app *application) rateLimitExceedResponse(w http.ResponseWriter, r *http.Request) {
	message := "request rate limit reached, please try again later"
	app.errorResponse(w, r, http.StatusTooManyRequests, message)
}

func (app *application) invalidActivationTokenResponse(w http.ResponseWriter, r *http.Request) {
	message := "invalid or expired activation token"
	app.errorResponse(w, r, http.StatusUnauthorized, message)
}

func (app *application) invalidAuthenticationCredResponse(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", "Bearer Jwt")
	message := "invalid authentication creds or token"
	app.errorResponse(w, r, http.StatusUnauthorized, message)
}

func (app *application) invalidJWTTokenSignatureResponse(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", "Bearer Jwt")
	message := "invalid jwt token signature."
	app.errorResponse(w, r, http.StatusUnauthorized, message)
}

func (app *application) authenticationRequiredResposne(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", "Bearer Jwt")
	message := "authentication required"
	app.errorResponse(w, r, http.StatusUnauthorized, message)
}

func (app *application) unauthorizedAccessInactiveUserResponse(w http.ResponseWriter, r *http.Request) {
	message := "user must be activated to access this resource"
	app.errorResponse(w, r, http.StatusForbidden, message)
}

func (app *application) notPermittedResponse(w http.ResponseWriter, r *http.Request) {
	message := "your user account doesn't have the necessary permissions to access this resource"
	app.errorResponse(w, r, http.StatusForbidden, message)
}
