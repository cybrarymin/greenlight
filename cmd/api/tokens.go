package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/cybrarymin/greenlight/internal/data"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

func (app *application) userActivationHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("userActivation.handler.tracer").Start(r.Context(), "userActivation.handler.span")
	defer span.End()

	userID, err := app.readUUIDParam(r)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, otelUserActivationFailureErr)
		app.failedValidationResponse(w, r, map[string]string{"uuid": "invalid uuid"})
		return
	}
	var input struct {
		UserToken string `json:"token"`
	}

	err = app.readJson(w, r, &input)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, otelunprocessableErr)
		app.badRequestResponse(w, r, err)
		return
	}

	nVal := data.NewValidator()
	if data.ValidateTokenPlaintext(nVal, input.UserToken); !nVal.Valid() {
		span.RecordError(errors.New(createKeyValuePairs(nVal.Errors)))
		span.SetStatus(codes.Error, otelunprocessableErr)
		app.failedValidationResponse(w, r, nVal.Errors)
		return
	}

	nTokens, err := app.models.Tokens.GetTokensOfUserID(ctx, userID, data.ActivationScope)
	if err != nil {
		span.RecordError(err)
		switch {
		case errors.Is(err, data.ErrorRecordNotFound):
			span.SetStatus(codes.Ok, otelDBNotFoundInfo)
			app.notFoundResponse(w, r)
			return
		default:
			span.SetStatus(codes.Ok, otelDBErr)
			app.serverErrorResponse(w, r, err)
			return
		}
	}
	matchedToken, ok := nTokens.Match(input.UserToken)
	if time.Now().After(matchedToken.Expiry) {
		span.RecordError(errors.New("token expired"))
		span.SetStatus(codes.Error, otelUserActivationFailureErr)
		app.invalidActivationTokenResponse(w, r)
		return
	}

	if !ok {
		span.SetStatus(codes.Error, otelUserActivationFailureErr)
		app.invalidActivationTokenResponse(w, r)
		return
	}

	matchedToken.User.Activated = true
	err = app.models.Users.Update(userID, ctx, matchedToken.User)
	if err != nil {
		span.RecordError(err)
		switch {
		case errors.Is(err, data.ErrorRecordNotFound):
			span.SetStatus(codes.Ok, otelDBNotFoundInfo)
			app.notFoundResponse(w, r)
			return
		case errors.Is(err, data.ErrEditConflict):
			span.SetStatus(codes.Error, otelDBErr)
			app.editConflictResponse(w, r)
			return
		default:
			span.SetStatus(codes.Error, otelDBErr)
			app.serverErrorResponse(w, r, err)
			return
		}
	}

	err = app.models.Tokens.DeleteAllForUser(ctx, userID, data.ActivationScope)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, otelDBErr)
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJson(w, http.StatusOK, envelope{"result": "user activated"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
