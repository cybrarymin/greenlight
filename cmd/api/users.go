package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/cybrarymin/greenlight/internal/data"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("registerUser.handler.tracer").Start(r.Context(), "registerUser.handler.span")
	span.End()

	nVal := data.NewValidator()

	var nInput struct {
		Name     string `json:"name"`
		Password string `json:"password"`
		Email    string `json:"email"`
	}

	err := app.readJson(w, r, &nInput)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, otelunprocessableErr)
		app.badRequestResponse(w, r, err)
		return
	}

	nUser := data.User{
		Name:      nInput.Name,
		Email:     nInput.Email,
		Activated: false, // new created users always are deactive
	}
	err = nUser.Password.Set(nInput.Password)
	if err != nil {
		span.RecordError(err)
		switch {
		case errors.Is(err, data.ErrorPasswordTooLong):
			span.SetStatus(codes.Error, otelunprocessableErr)
			app.badRequestResponse(w, r, err)
			return
		default:
			span.SetStatus(codes.Error, "error on new password setup")
			app.serverErrorResponse(w, r, err)
			return
		}
	}

	data.ValidateUser(nVal, &nUser)
	valid := nVal.Valid()
	if !valid {
		span.RecordError(errors.New(createKeyValuePairs(nVal.Errors)))
		span.SetStatus(codes.Error, otelunprocessableErr)
		app.failedValidationResponse(w, r, nVal.Errors)
		return
	}

	err = app.models.Users.Insert(ctx, &nUser)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, otelDBErr)
		switch {
		case errors.Is(err, data.ErrorDuplicateEmail):
			nVal.AddError("email", "user with current email already exists")
			app.failedValidationResponse(w, r, nVal.Errors)
			return
		default:
			app.serverErrorResponse(w, r, err)
			return
		}
	}

	err = app.models.Permissions.AddPermForUser(ctx, nUser.ID, "movies:read")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, otelDBErr)
		app.serverErrorResponse(w, r, err)
		return
	}

	app.BackgroundJob(func() {

		nToken, err := app.models.Tokens.New(ctx, time.Hour*72, nUser.ID, data.ActivationScope)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, otelDBErr)
			app.log.Error().Err(err).Msg(fmt.Sprintf("token creation procedure failed for user %v", nUser.Email))
			return
		}

		mailData := struct {
			ID   string
			Code string
		}{
			ID:   nUser.ID.String(),
			Code: nToken.PlainText,
		}
		// retrying email sending if it failed
		for i := 0; i < 3; i++ {
			err = app.mailer.Send(nUser.Email, "user_welcome.tpl", mailData)
			if err == nil {
				return
			} else {
				app.log.Error().Err(err).Msg(fmt.Sprintf("failed to send email to user %v", nUser.Email))
				time.Sleep(500 * time.Millisecond)
			}
		}
	}, "panic happened during sending email to user for activation")

	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/v1/users/%d", nUser.ID))
	err = app.writeJson(w, http.StatusAccepted, envelope{"result": nUser}, headers)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

}

func (app *application) ListUserHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("listUser.handler.tracer").Start(r.Context(), "listUser.handler.span")
	defer span.End()
	nValidator := data.NewValidator()
	var input struct {
		Name  string
		Email string
		data.Filters
	}
	qs := r.URL.Query()
	input.Filters.Page = app.readInt(qs, "page", 1, nValidator)
	input.Filters.PageSize = app.readInt(qs, "page_size", 100, nValidator)
	input.Filters.Sort = app.readString(qs, "sort", "id")
	input.Filters.SortSafeList = []string{"id", "created_at", "name", "email", "-id", "-created_at", "-name", "-email"}
	input.Name = app.readString(qs, "name", "")
	input.Email = app.readString(qs, "email", "")
	input.Filters.ValidateFilters(nValidator)
	if !nValidator.Valid() {
		span.RecordError(errors.New(createKeyValuePairs(nValidator.Errors)))
		span.SetStatus(codes.Error, otelunprocessableErr)
		app.failedValidationResponse(w, r, nValidator.Errors)
		return
	}

	userList := &data.Users{}
	count, err := app.models.Users.List(ctx, userList, input.Name, input.Email, &input.Filters)
	if err != nil {
		span.RecordError(err)
		switch {
		case errors.Is(err, data.ErrorRecordNotFound):
			span.SetStatus(codes.Ok, otelDBNotFoundInfo)
		default:
			span.SetStatus(codes.Error, otelDBErr)
			app.serverErrorResponse(w, r, err)
			return
		}
	}
	pMeta := input.Filters.PaginationMetaData(ctx, count)
	err = app.writeJson(w, http.StatusOK, envelope{"Metadata": pMeta, "Result": userList}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}

func (app *application) DeleteUserHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("deleteUser.handler.tracer").Start(r.Context(), "deleteUser.handler.span")
	defer span.End()
	uuid, err := app.readUUIDParam(r)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, otelunprocessableErr)
		app.badRequestResponse(w, r, err)
		return
	}
	err = app.models.Users.Delete(ctx, uuid)
	if err != nil {
		span.RecordError(err)
		switch {
		case errors.Is(err, data.ErrorRecordNotFound):
			span.SetStatus(codes.Ok, otelDBNotFoundInfo)
			app.notFoundResponse(w, r)
			return
		default:
			span.SetStatus(codes.Error, otelDBErr)
			app.serverErrorResponse(w, r, err)
			return
		}
	}
	err = app.writeJson(w, http.StatusOK, envelope{"result": "user deleted successfully"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
