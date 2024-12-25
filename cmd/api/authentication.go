package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/cybrarymin/greenlight/internal/data"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var JWTKEY string

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
