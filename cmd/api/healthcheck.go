package api

import (
	"net/http"

	"go.opentelemetry.io/otel"
)

func (app *application) healthcheckHandler(w http.ResponseWriter, r *http.Request) {
	_, span := otel.Tracer("healthcheck.handler.tracer").Start(r.Context(), "healthcheck.handler.span")
	defer span.End()
	data := map[string]string{
		"status":      "available",
		"environment": Env,
		"version":     Version,
	}
	err := app.writeJson(w, http.StatusOK, envelope{
		"health": data,
	}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}
