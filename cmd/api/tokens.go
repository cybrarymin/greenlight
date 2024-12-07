package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/cybrarymin/greenlight/internal/data"
)

func (app *application) userActivationHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	userID, err := app.readUUIDParam(r)
	if err != nil {
		app.failedValidationResponse(w, r, map[string]string{"uuid": "invalid uuid"})
		return
	}
	var input struct {
		UserToken string `json:"token"`
	}

	err = app.readJson(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	nVal := data.NewValidator()
	if data.ValidateTokenPlaintext(nVal, input.UserToken); !nVal.Valid() {
		app.failedValidationResponse(w, r, nVal.Errors)
		return
	}

	nTokens, err := app.models.Tokens.GetTokensOfUserID(ctx, userID, data.ActivationScope)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrorRecordNotFound):
			app.notFoundResponse(w, r)
			return
		default:
			app.serverErrorResponse(w, r, err)
			return
		}
	}
	matchedToken, ok := nTokens.Match(input.UserToken)
	if time.Now().After(matchedToken.Expiry) {
		app.invalidActivationTokenResponse(w, r)
		return
	}

	if !ok {
		app.invalidActivationTokenResponse(w, r)
		return
	}

	matchedToken.User.Activated = true
	err = app.models.Users.Update(userID, ctx, matchedToken.User)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrorRecordNotFound):
			app.notFoundResponse(w, r)
			return
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
			return
		default:
			app.serverErrorResponse(w, r, err)
			return
		}
	}

	err = app.models.Tokens.DeleteAllForUser(ctx, userID, data.ActivationScope)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJson(w, http.StatusOK, envelope{"result": "user activated"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
