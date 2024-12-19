package api

import (
	"context"
	"net/http"

	"github.com/cybrarymin/greenlight/internal/data"
)

type contextKey string

const userContextKey = contextKey("user")

func (app *application) SetUserContext(r *http.Request, u *data.User) *http.Request {
	ctx := context.WithValue(r.Context(), userContextKey, u)
	return r.WithContext(ctx)
}

func (app *application) GetUserContext(r *http.Request) *data.User {
	user, ok := r.Context().Value(userContextKey).(*data.User)
	if !ok {
		panic("missing user value in http request context")
	}
	return user
}
