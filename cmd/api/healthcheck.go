package api

import (
	"net/http"
)

func (app *application) healthcheckHandler(w http.ResponseWriter, r *http.Request) {
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
