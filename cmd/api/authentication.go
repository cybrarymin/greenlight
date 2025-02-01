package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/cybrarymin/greenlight/internal/data"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

var JWTKEY string

func (app *application) createBearerTokenHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("createBearerToken.handler.tracer").Start(r.Context(), "createBearerToken.handler.span")
	defer span.End()

	ok, nUser := app.BasicAuth(w, r)
	if !ok {
		return
	}
	nBToken, err := app.models.Tokens.New(ctx, time.Hour*24, nUser.ID, data.AuthenticationScope)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, otelDBErr)
		app.serverErrorResponse(w, r, err)
		return
	}
	err = app.writeJson(w, http.StatusCreated, envelope{"result": nBToken}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

}

type customClaims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

/*
This function is used comletely to implement jwt.claimsValidator.
When we define this function for our customClaim then jwt.Validator will validate our custom claim after the registered claim based on this function
*/
func (c *customClaims) Validate() error {
	if ok := data.EmailRX.MatchString(c.Email); !ok {
		return errors.New("invalid email claim on jwt token")
	}
	return nil
}

/*
Authenticating user using basic authentication method. If user is valid it's gonna issue a JWT Token to the user
*/
func (app *application) createJWTTokenHandler(w http.ResponseWriter, r *http.Request) {
	_, span := otel.Tracer("createJWTToken.handler.tracer").Start(r.Context(), "createJWTToken.handler.span")
	defer span.End()

	ok, nUser := app.BasicAuth(w, r)
	if !ok {
		return
	}
	claims := customClaims{
		Email: nUser.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "greenlight.example.com",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24 * 3)),
			Subject:   nUser.Email,
			Audience:  []string{"greenlight.example.com"},
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        uuid.New().String(),
		},
	}
	span.SetAttributes(attribute.String("claims.email", claims.Email))
	span.SetAttributes(attribute.String("claims.issuer", claims.Issuer))
	span.SetAttributes(attribute.String("claims.subject", claims.Subject))
	span.SetAttributes(attribute.StringSlice("claims.audience", claims.Audience))
	span.SetAttributes(attribute.String("claims.id", claims.ID))

	jToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims, func(t *jwt.Token) {})

	signedToken, err := jToken.SignedString([]byte(JWTKEY))
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	err = app.writeJson(w, http.StatusOK, envelope{"result": map[string]string{"token": signedToken}}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}

/*
Authenticates the user using basic authentication method.
in case of successfull authentication it returns ok plus userinfo
*/
func (app *application) BasicAuth(w http.ResponseWriter, r *http.Request) (bool, *data.User) {
	ctx, span := otel.Tracer("basicAuth.handler.tracer").Start(r.Context(), "basicAuth.handler.span")
	defer span.End()

	email, pass, ok := r.BasicAuth()
	if !ok {
		span.SetStatus(codes.Error, otelAuthFailureErr)
		app.invalidAuthenticationCredResponse(w, r)
		return false, nil
	}

	nValidator := data.NewValidator()
	data.ValidateEmail(nValidator, email)
	data.ValidatePasswordPlaintext(nValidator, pass)
	if !nValidator.Valid() {
		span.RecordError(errors.New(createKeyValuePairs(nValidator.Errors)))
		span.SetStatus(codes.Error, otelAuthFailureErr)
		app.invalidAuthenticationCredResponse(w, r)
		return false, nil
	}

	nUser, err := app.models.Users.GetByEmail(email, ctx)
	if err != nil {
		span.RecordError(err)
		switch {
		case errors.Is(err, data.ErrorRecordNotFound):
			span.SetStatus(codes.Error, otelAuthFailureErr)
			app.invalidActivationTokenResponse(w, r)
			return false, nil
		default:
			span.SetStatus(codes.Error, otelDBErr)
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
		span.RecordError(err)
		span.SetStatus(codes.Error, otelAuthFailureErr)
		app.serverErrorResponse(w, r, err)
		return false, nil
	}
	if !ok && err == nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("user.email", email))
		span.SetStatus(codes.Error, otelAuthFailureErr)
		app.invalidAuthenticationCredResponse(w, r)
		return false, nil
	}

	return true, nUser
}
