package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/cybrarymin/greenlight/internal/data"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (app *application) createMovieHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("createMovie.handler.tracer").Start(r.Context(), "createMovie.handler.span")
	defer span.End()

	var input struct {
		Title   string
		Year    int32
		Runtime data.Runtime
		Genres  []string
	}
	err := app.readJson(w, r, &input)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, otelunprocessableErr)
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
		span.RecordError(errors.New(createKeyValuePairs(nvalidator.Errors)))
		span.SetStatus(codes.Error, otelunprocessableErr)
		app.errorResponse(w, r, http.StatusUnprocessableEntity, nvalidator.Errors)
		return
	}

	span.AddEvent("inserting movie to the database", trace.WithAttributes(
		attribute.String("movie.title", movie.Title),
	))
	err = app.models.Movies.Insert(ctx, &movie)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, otelDBErr)
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

func (app *application) listMovieHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("listMovie.handler.tracer").Start(r.Context(), "listMovie.handler.span")
	defer span.End()

	var input struct {
		Title  string
		Genres []string
		data.Filters
	}

	span.AddEvent("reading and validating query parameters")
	v := data.NewValidator()
	qs := r.URL.Query()
	input.Title = app.readString(qs, "title", "")
	input.Genres = app.readCSV(qs, "genres", []string{})
	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 20, v)
	input.Filters.Sort = app.readString(qs, "sort", "id")
	input.Filters.SortSafeList = []string{"id", "title", "year", "runtime", "-id", "-title", "-year", "-runtime"}
	input.Filters.ValidateFilters(v)
	if !v.Valid() {
		span.RecordError(errors.New(createKeyValuePairs(v.Errors)))
		span.SetStatus(codes.Error, otelunprocessableErr)
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	span.AddEvent("querying database to get list of movies")
	movies, count, err := app.models.Movies.List(ctx, input.Title, input.Genres, &input.Filters)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrorRecordNotFound):
			span.RecordError(err)
			span.SetStatus(codes.Ok, otelDBNotFoundInfo)
			app.notFoundResponse(w, r)
		default:
			span.RecordError(err)
			span.SetStatus(codes.Error, otelDBErr)
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	pMeta := input.Filters.PaginationMetaData(ctx, count)

	err = app.writeJson(w, http.StatusOK, envelope{"Metadata": pMeta, "Movies": movies}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}
func (app *application) showMovieHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("showMovie.handler.tracer").Start(r.Context(), "showMovie.handler.span")
	defer span.End()

	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	span.AddEvent("fetching movie information from database", trace.WithAttributes(attribute.Int64("movie.id", id)))
	movie, err := app.models.Movies.Select(ctx, id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrorRecordNotFound):
			span.RecordError(err)
			span.SetStatus(codes.Ok, otelDBNotFoundInfo)
			app.notFoundResponse(w, r)
		default:
			span.RecordError(err)
			span.SetStatus(codes.Error, otelDBErr)
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
	ctx, span := otel.Tracer("showMovie.handler.tracer").Start(r.Context(), "showMovie.handler.span")
	defer span.End()

	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	span.AddEvent("deleting the movie from database", trace.WithAttributes(attribute.Int64("movie.id", id)))
	err = app.models.Movies.Delete(ctx, id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrorRecordNotFound):
			span.RecordError(err)
			span.SetStatus(codes.Ok, otelDBNotFoundInfo)
			app.notFoundResponse(w, r)
		default:
			span.RecordError(err)
			span.SetStatus(codes.Error, otelDBErr)
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
	ctx, span := otel.Tracer("showMovie.handler.tracer").Start(r.Context(), "showMovie.handler.span")
	defer span.End()

	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
	}

	span.AddEvent("fetching the movie information from database to update", trace.WithAttributes(attribute.Int64("movie.id", id)))
	nMovie, err := app.models.Movies.Select(ctx, id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrorRecordNotFound):
			span.RecordError(err)
			span.SetStatus(codes.Ok, otelDBNotFoundInfo)
			app.notFoundResponse(w, r)
		default:
			span.RecordError(err)
			span.SetStatus(codes.Error, otelDBErr)
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	var input struct {
		Title   *string
		Year    *int32
		Runtime *data.Runtime
		Genres  *[]string
	}

	err = app.readJson(w, r, &input)
	if err != nil {
		span.SetStatus(codes.Error, otelunprocessableErr)
		app.badRequestResponse(w, r, err)
		return
	}

	if input.Title != nil {
		nMovie.Title = *input.Title
	}

	if input.Year != nil {
		nMovie.Year = *input.Year
	}

	if input.Runtime != nil {
		nMovie.Runtime = *input.Runtime
	}

	if input.Genres != nil {
		nMovie.Genres = *input.Genres
	}
	nvalidator := data.NewValidator()
	nMovie.Validator(nvalidator)
	if len(nvalidator.Errors) > 0 {
		span.RecordError(errors.New(createKeyValuePairs(nvalidator.Errors)))
		span.SetStatus(codes.Error, otelunprocessableErr)
		app.failedValidationResponse(w, r, nvalidator.Errors)
		return
	}
	span.AddEvent("updating the movie in database", trace.WithAttributes(attribute.Int64("movie.id", id)))
	err = app.models.Movies.Update(context.Background(), nMovie.ID, nMovie)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, otelDBErr)
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJson(w, http.StatusOK, envelope{"result": nMovie}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

}
