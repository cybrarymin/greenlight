package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/cybrarymin/greenlight/internal/data"
)

func (app *application) createMovieHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title   string
		Year    int32
		Runtime data.Runtime
		Genres  []string
	}
	err := app.readJson(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	movie := data.Movie{
		Title:   input.Title,
		Year:    input.Year,
		Runtime: input.Runtime,
		Genres:  input.Genres,
	}
	nvalidator := data.NewValidator()
	movie.Validator(nvalidator)
	if len(nvalidator.Errors) > 0 {
		app.errorResponse(w, r, http.StatusUnprocessableEntity, nvalidator.Errors)
		return
	}

	err = app.models.Movies.Insert(context.Background(), &movie)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/v1/movies/%d", movie.ID))
	err = app.writeJson(w, http.StatusCreated, envelope{"result": movie}, headers)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

}

func (app *application) showMovieHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	movie, err := app.models.Movies.Select(context.Background(), id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrorRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJson(w, http.StatusOK, envelope{"Movie": movie}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

func (app *application) deleteMovieHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	err = app.models.Movies.Delete(context.Background(), id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrorRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	err = app.writeJson(w, http.StatusOK, envelope{"result": "movie deleted successfully"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

func (app *application) updateMovieHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
	}

	nMovie, err := app.models.Movies.Select(context.Background(), id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrorRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	var input struct {
		Title   string
		Year    int32
		Runtime data.Runtime
		Genres  []string
	}

	err = app.readJson(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
	}

	nMovie.Title = input.Title
	nMovie.Year = input.Year
	nMovie.Runtime = input.Runtime
	nMovie.Genres = input.Genres

	nvalidator := data.NewValidator()
	nMovie.Validator(nvalidator)
	if len(nvalidator.Errors) > 0 {
		app.failedValidationResponse(w, r, nvalidator.Errors)
		return
	}

	err = app.models.Movies.Update(context.Background(), nMovie.ID, nMovie)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJson(w, http.StatusOK, envelope{"result": nMovie}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

}
