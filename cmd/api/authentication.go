package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/cybrarymin/greenlight/internal/data"
)

func (app *application) createBearerTokenHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	ok, nUser := app.BasicAuth(w, r)
	if !ok {
		return
	}
	nBToken, err := app.models.Tokens.New(ctx, time.Hour*24, nUser.ID, data.AuthenticationScope)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	err = app.writeJson(w, http.StatusCreated, envelope{"result": nBToken}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

}

func (app *application) BasicAuth(w http.ResponseWriter, r *http.Request) (bool, *data.User) {
	ctx := context.Background()
	email, pass, ok := r.BasicAuth()
	if !ok {
		app.invalidAuthenticationCredResponse(w, r)
		return false, nil
	}

	nValidator := data.NewValidator()
	data.ValidateEmail(nValidator, email)
	data.ValidatePasswordPlaintext(nValidator, pass)
	if !nValidator.Valid() {
		app.invalidAuthenticationCredResponse(w, r)
		return false, nil
	}

	nUser, err := app.models.Users.GetByEmail(email, ctx)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrorRecordNotFound):
			app.invalidActivationTokenResponse(w, r)
			return false, nil
		default:
			app.serverErrorResponse(w, r, err)
			return false, nil
		}
	}

	inputPass := data.Password{
		Plaintext: &pass,
		Hash:      nUser.Password.Hash, // Considering the database hash to check the validity of the password
	}

	ok, err = inputPass.Match()
	if !ok && err != nil {
		app.serverErrorResponse(w, r, err)
		return false, nil
	}
	if !ok && err == nil {
		app.invalidAuthenticationCredResponse(w, r)
		return false, nil
	}

	return true, nUser
}
