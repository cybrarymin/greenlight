package api

import (
	_ "github.com/cybrarymin/greenlight/docs"
	"github.com/cybrarymin/greenlight/internal/data"
)

// @title			Documentation of greenlight app api
// @version		1
// @description	api documentation
// @contact.name	Ryan
// @contact.url	https://github.com/cybrarymin
// @contact.email	aminmoghaddam1377@gmail.com
// @host			127.0.0.1:8080
// @basepath		/v1
type SwaggerGetResponse struct {
	Movie data.Movie `json:"Movie"`
}

type SwaggerCreateMovieInput struct {
	Title   string   `json:"title"   example:"avengers"`
	Year    int32    `json:"year"    example:"2018"`
	Runtime string   `json:"runtime" example:"75 mins"` // or use a custom type
	Genres  []string `json:"genres"  example:"adventure,action"`
}

type SwaggerCreateResponse struct {
	Result data.Movie
}

type SwaggerDeleteResponse struct {
	Result string `json:"result" example:"movie deleted successfully"`
}

type SwaggerListResponse struct {
	Metadata data.PaginationMeta
	Movies   []data.Movie
}

type SwaggerNotFound struct {
	Error string `json:"error" example:"the requested resource couldn't be found"`
}

type SwaggerServerErrorResponse struct {
	Error string `json:"error" example:"the server encountered an error to process the request"`
}

type SwaggerBadRequestResponse struct {
	Error string `json:"error" example:"bad request error"`
}

type SwaggerFailedValidationResponse struct {
	Error string `json:"error" example:"unprocessable input error"`
}

type SwaggerEditConflictResponse struct {
	Error string `json:"error" example:"unable to update the record due to an edit conflict, please try again"`
}

type SwaggerRateLimitExceedResponse struct {
	Error string `json:"error" example:"request rate limit reached, please try again later"`
}

type SwaggerUnauthorizaed struct {
	Error string `json:"error" example:"unauthorized request"`
}

type SwaggerNotPermitted struct {
	Error string `json:"error" example:"permission denied"`
}
