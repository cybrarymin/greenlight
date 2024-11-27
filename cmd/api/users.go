package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/cybrarymin/greenlight/internal/data"
)

func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {
	nVal := data.NewValidator()

	var nInput struct {
		Name     string `json:name`
		Password string `json:password`
		Email    string `json:email`
	}

	err := app.readJson(w, r, &nInput)
	if err != nil {
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
		switch {
		case errors.Is(err, data.ErrorPasswordTooLong):
			app.badRequestResponse(w, r, err)
			return
		default:
			app.serverErrorResponse(w, r, err)
			return
		}
	}

	data.ValidateUser(nVal, &nUser)
	valid := nVal.Valid()
	if !valid {
		app.failedValidationResponse(w, r, nVal.Errors)
		return
	}

	err = app.models.Users.Insert(context.Background(), &nUser)
	if err != nil {
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

	err = app.mailer.Send(nUser.Email, "user_welcome.tpl", nUser)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/v1/users/%d", nUser.ID))
	err = app.writeJson(w, http.StatusCreated, envelope{"result": nUser}, headers)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

}

func (app *application) ListUserHandler(w http.ResponseWriter, r *http.Request) {
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

	userList := &data.Users{}
	count, err := app.models.Users.List(context.Background(), userList, input.Name, input.Email, &input.Filters)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrorRecordNotFound):
			break
		default:
			app.serverErrorResponse(w, r, err)
			return
		}
	}
	pMeta := input.Filters.PaginationMetaData(count)
	err = app.writeJson(w, http.StatusOK, envelope{"Metadata": pMeta, "Result": userList}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}

func (app *application) DeleteUserHandler(w http.ResponseWriter, r *http.Request) {
	uuid, err := app.readUUIDParam(r)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	err = app.models.Users.Delete(context.Background(), uuid)
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
	err = app.writeJson(w, http.StatusOK, envelope{"result": "user deleted successfully"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
