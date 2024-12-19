package api

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cybrarymin/greenlight/internal/data"
	"github.com/felixge/httpsnoop"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/time/rate"
)

type ClientRateLimiter struct {
	Limit      *rate.Limiter
	LastAccess *time.Timer
}

func (app *application) PanicRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This deferred anonymous function will be run after panic is happening
		defer func() {
			// recover() will stop panic to close the program and instead returns error status 500 internal server error to the client
			if panicErr := recover(); panicErr != nil {
				// Setting this header will trigger the HTTP server to close the connection after Panic happended
				w.Header().Set("Connection", "close")
				app.serverErrorResponse(w, r, fmt.Errorf("%s, %s", panicErr, debug.Stack()))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (app *application) RateLimit(next http.Handler) http.Handler {
	if app.config.rateLimit.enabled {
		// Global rate limiter
		busrtSize := app.config.rateLimit.globalRateLimit + app.config.rateLimit.globalRateLimit/10
		nRL := rate.NewLimiter(rate.Limit(app.config.rateLimit.globalRateLimit), int(busrtSize))
		// Per IP or Per Client rate limiter
		pcbusrtSize := app.config.rateLimit.perClientRateLimit + app.config.rateLimit.perClientRateLimit/10
		pcnRL := make(map[string]ClientRateLimiter)
		mu := sync.RWMutex{}
		expirationTime := 30 * time.Second

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !nRL.Allow() { // In this code, whenever we call the Allow() method on the rate limiter exactly one token will be consumed from the bucket. And if there is no token in the bucket left Allow() will return false
				app.rateLimitExceedResponse(w, r)
				return
			}
			clientAddr, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				app.serverErrorResponse(w, r, err)
				return
			}
			mu.RLock()
			if _, found := pcnRL[clientAddr]; !found {

				pcnRL[clientAddr] = ClientRateLimiter{
					rate.NewLimiter(rate.Limit(app.config.rateLimit.perClientRateLimit), int(pcbusrtSize)),
					time.NewTimer(expirationTime),
				}
				mu.RUnlock()

				go func() {
					<-pcnRL[clientAddr].LastAccess.C
					mu.Lock()
					delete(pcnRL, clientAddr)
					mu.Unlock()
				}()

			} else {
				app.log.Debug().Msgf("renewing client %v expiry of rate limiting context", clientAddr)
				pcnRL[clientAddr].LastAccess.Reset(expirationTime)
			}

			mu.RLock()
			if !pcnRL[clientAddr].Limit.Allow() {
				app.rateLimitExceedResponse(w, r)
				return
			}
			mu.RUnlock()

			next.ServeHTTP(w, r)
		})
	} else {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}

func (app *application) Auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		headerValue := r.Header.Get("Authorization")

		if headerValue == "" {
			r = app.SetUserContext(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		headerValues := strings.Split(headerValue, " ")

		if len(headerValues) != 2 || headerValues[0] != "Bearer" {
			app.invalidAuthenticationCredResponse(w, r)
			return
		}
		userToken := headerValues[1]

		nValidator := data.NewValidator()
		data.ValidateTokenPlaintext(nValidator, userToken)
		if !nValidator.Valid() {
			app.invalidActivationTokenResponse(w, r)
			return
		}

		user, err := app.models.Users.GetUserByToken(ctx, userToken, data.AuthenticationScope)
		if err != nil {
			switch {
			case errors.Is(err, data.ErrorRecordNotFound):
				app.invalidAuthenticationCredResponse(w, r)
				return
			default:
				app.serverErrorResponse(w, r, err)
				return
			}
		}
		r = app.SetUserContext(r, user)

		next.ServeHTTP(w, r)
	}
}
func (app *application) requiredNonAnonymousUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nUser := app.GetUserContext(r)
		if nUser.IsAnonymous() {
			app.authenticationRequiredResposne(w, r)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func (app *application) requireActivatedUser(next http.HandlerFunc) http.HandlerFunc {
	// defining fn as a function
	fn := func(w http.ResponseWriter, r *http.Request) {
		nUser := app.GetUserContext(r)
		if !nUser.Activated {
			app.unauthorizedAccessInactiveUserResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	}
	return app.requiredNonAnonymousUser(fn)
}

func (app *application) requirePermission(reqPermission string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		nUser := app.GetUserContext(r)

		perms, err := app.models.Permissions.GetAllPermsForUser(ctx, nUser.ID)
		if err != nil {
			switch {
			case errors.Is(err, data.ErrorRecordNotFound):
				app.notPermittedResponse(w, r)
				return
			default:
				app.serverErrorResponse(w, r, err)
				return
			}
		}
		ok := perms.IncludesPrem(reqPermission)
		if !ok {
			app.notPermittedResponse(w, r)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func (app *application) enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		next.ServeHTTP(w, r)
	})
}

func (app *application) promMetrics(path string, next http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pTimer := prometheus.NewTimer(promHttpDuration.WithLabelValues(path))
		promHttpTotalRequests.WithLabelValues(path).Inc()
		metrics := httpsnoop.CaptureMetrics(next, w, r)
		promHttpTotalResponse.WithLabelValues().Inc()
		promHttpResponseStatus.WithLabelValues(strconv.Itoa(metrics.Code)).Inc()
		pTimer.ObserveDuration()
	})
}
