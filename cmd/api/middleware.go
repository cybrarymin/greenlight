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
	"github.com/golang-jwt/jwt/v5"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
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
		ctx, span := otel.Tracer("auth.handler.tracer").Start(r.Context(), "auth.handler.span")
		defer span.End()
		headerValue := r.Header.Get("Authorization")

		if headerValue == "" {
			span.AddEvent("starting request with anonymous user")
			r = app.SetUserContext(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		headerValues := strings.Split(headerValue, " ")

		if len(headerValues) != 2 || headerValues[0] != "Bearer" {
			span.SetStatus(codes.Error, "Invalid authentication header in request")
			app.invalidAuthenticationCredResponse(w, r)
			return
		}
		userToken := headerValues[1]

		nValidator := data.NewValidator()
		data.ValidateTokenPlaintext(nValidator, userToken)
		if !nValidator.Valid() {
			span.SetStatus(codes.Error, "Invalid authentication token")
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
				span.RecordError(err)
				span.SetStatus(codes.Error, otelDBErr)
				app.serverErrorResponse(w, r, err)
				return
			}
		}
		r = r.WithContext(ctx)
		r = app.SetUserContext(r, user)

		next.ServeHTTP(w, r)
	}
}

func (app *application) JWTAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		headerValue := r.Header.Get("Authorization")
		if headerValue == "" {
			r = app.SetUserContext(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		headerValues := strings.Split(headerValue, " ")
		if len(headerValues) != 2 && headerValues[0] != "Bearer" {
			app.invalidAuthenticationCredResponse(w, r)
			return
		}
		jToken := headerValues[1]
		// ParseWithClaims will fetch the token and keystring of the token
		// It will verify the signature to make sure token is valid
		// It will verify all the registered claims of jwt.Registered claims
		verifiedToken, err := jwt.ParseWithClaims(jToken, &customClaims{}, func(t *jwt.Token) (interface{}, error) {
			return []byte(JWTKEY), nil
		})
		if err != nil {
			switch {
			case errors.Is(err, jwt.ErrTokenSignatureInvalid):
				app.invalidJWTTokenSignatureResponse(w, r)
				return
			default:
				app.invalidAuthenticationCredResponse(w, r)
				return
			}
		}
		if !verifiedToken.Valid {
			app.invalidAuthenticationCredResponse(w, r)
			return
		}

		user, err := app.models.Users.GetByEmail(verifiedToken.Claims.(*customClaims).Email, ctx)
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
		ctx, span := otel.Tracer("requireActivatedUser.handler.tracer").Start(r.Context(), "requireActivatedUser.handler.span")
		defer span.End()

		nUser := app.GetUserContext(r)
		span.AddEvent("checking user activation status", trace.WithAttributes(
			attribute.String("user.Email", nUser.Email),
			attribute.String("user.Name", nUser.Name),
		))
		if !nUser.Activated {
			app.unauthorizedAccessInactiveUserResponse(w, r)
			return
		}

		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	}
	return app.requiredNonAnonymousUser(fn)
}

func (app *application) requirePermission(reqPermission string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := otel.Tracer("requirepermission.handler.tracer").Start(r.Context(), "requirepermission.handler.span")
		defer span.End()

		nUser := app.GetUserContext(r)

		perms, err := app.models.Permissions.GetAllPermsForUser(ctx, nUser.ID)
		if err != nil {
			switch {
			case errors.Is(err, data.ErrorRecordNotFound):
				span.AddEvent("no record found",
					trace.WithAttributes(attribute.String("user.email", nUser.Email)),
				)
				app.notPermittedResponse(w, r)
				return
			default:
				span.RecordError(err)
				span.SetStatus(codes.Error, otelDBErr)
				app.serverErrorResponse(w, r, err)
				return
			}
		}
		ok := perms.IncludesPrem(reqPermission)
		if !ok {
			app.notPermittedResponse(w, r)
			return
		}

		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	}
}

func (app *application) enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Api_Key, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTION, HEAD")
		next.ServeHTTP(w, r)
	})
}

func (app *application) promMetrics(path string, next http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This will consider a timer for histogram and summary metric times
		// defer function will will expose the metrics sine the timer has been set.
		pTimer := prometheus.NewTimer(promHttpDuration.WithLabelValues(path))
		defer pTimer.ObserveDuration()
		promHttpTotalRequests.WithLabelValues(path).Inc()
		metrics := httpsnoop.CaptureMetrics(next, w, r)
		promHttpTotalResponse.WithLabelValues().Inc()
		promHttpResponseStatus.WithLabelValues(strconv.Itoa(metrics.Code)).Inc()

	})
}

func (app *application) otelHandler(next http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// using otelhttp default package to wrap the handler instead of creating a handler ourselves from scratch
		instrument := otelhttp.NewHandler(next, "otel.instrumented.handler")
		otelMetricHTTPTotalRequests.Add(r.Context(), 1,
			metric.WithAttributes(attribute.String("path", r.URL.Path)),
			metric.WithAttributes(attribute.String("method", r.Method)),
		)
		snoopMetrics := httpsnoop.CaptureMetrics(instrument, w, r)

		// http response time based on status codes
		otelMetricHttpDuration.Record(r.Context(), snoopMetrics.Duration.Seconds(),
			metric.WithAttributes(attribute.String("path", r.URL.Path)),
		)

		// http total responses
		otelMetricHTTPTotalResponses.Add(r.Context(), 1)
		// http total responses based on code
		otelMetricHTTPTotalResponseStatus.Add(r.Context(), 1,
			metric.WithAttributes(attribute.String("status", strconv.Itoa(snoopMetrics.Code))),
		)
	})
}
