package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"quizzivy/gen/openapi"
	"quizzivy/internal/httpx"
	"quizzivy/internal/join"
	"quizzivy/internal/ratelimit"
)

// RateLimits declares the policy for every public operation, plus the one
// authenticated operation that mints a credential.
//
// Numbers are §6.5's. The per-code buckets are wired in T-1.7, when the join
// endpoints learn how to read a code out of the request; the mechanism is here
// and tested.
//
// Adding a public operation to api/openapi.yaml without adding it here makes
// the server refuse to start -- see httpx.AssertPublicRoutesLimited.
func RateLimits() *ratelimit.Registry {
	reg := ratelimit.NewRegistry()
	const capacity = 10_000
	const maxKeyBodyBytes = 8 * 1024
	reg.Add("POST /join/preview", capacity, ratelimit.PerMinute(10), ratelimit.PerHour(60)).
		WithKey(ratelimit.JSONFieldKeyFunc("joinCode", maxKeyBodyBytes, join.Normalize), capacity, ratelimit.PerHour(30))
	reg.Add("POST /auth/google", capacity, ratelimit.PerMinute(10), ratelimit.PerHour(60)).
		WithKey(ratelimit.JSONFieldKeyFunc("joinCode", maxKeyBodyBytes, join.Normalize), capacity, ratelimit.PerHour(30))
	reg.Add("POST /auth/login", capacity, ratelimit.PerMinute(10), ratelimit.PerHour(60)).
		WithKey(ratelimit.JSONFieldKey("email", maxKeyBodyBytes), capacity, ratelimit.PerHour(20))
	reg.Add("POST /auth/refresh", capacity, ratelimit.PerMinute(30), ratelimit.PerHour(200))
	reg.Add("POST /auth/logout", capacity, ratelimit.PerMinute(30), ratelimit.PerHour(200))
	reg.Add("POST /app/classes/join", capacity, ratelimit.PerMinute(10), ratelimit.PerHour(60)).
		WithKey(ratelimit.JSONFieldKeyFunc("joinCode", maxKeyBodyBytes, join.Normalize), capacity, ratelimit.PerHour(30))
	reg.Add("POST /app/attempts/{id}/events", capacity, ratelimit.PerMinute(120))

	// Not public -- bearer-protected, so AssertPublicRoutesLimited would never
	// have asked for it -- but it is the only endpoint that mints a password.
	// A stolen admin session should not be able to grind out resets across a
	// roster faster than a teacher would ever need to, and one teacher does not
	// legitimately reset thirty accounts in an hour.
	reg.Add("POST /admin/students/{id}/reset-password", capacity,
		ratelimit.PerMinute(5), ratelimit.PerHour(30))

	return reg
}

// NewRouter builds the HTTP handler.
func NewRouter(deps Deps, logger *slog.Logger, allowedOrigins []string, clientIPHeader string) (http.Handler, error) {
	spec, err := openapi.GetSpec()
	if err != nil {
		return nil, err
	}

	limits := RateLimits()
	// Refuses to start rather than shipping an unprotected public endpoint.
	if err := httpx.AssertPublicRoutesLimited(spec, limits); err != nil {
		return nil, err
	}
	// Everything the contract does not explicitly open requires a bearer token.
	openRoutes := httpx.OpenRoutes(spec, "bearerAuth")

	validate, err := httpx.ValidateRequests(spec)
	if err != nil {
		return nil, err
	}

	server := &Server{Deps: deps}
	strict := openapi.NewStrictHandlerWithOptions(server, nil, openapi.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			httpx.WriteError(w, r, http.StatusBadRequest, httpx.CodeValidationFailed, err.Error())
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			if errors.Is(err, httpx.ErrNotImplemented) {
				httpx.WriteError(w, r, http.StatusNotImplemented, httpx.CodeInternal,
					"Chức năng này chưa được xây dựng.")
				return
			}
			logger.Error("handler", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
			httpx.WriteError(w, r, http.StatusInternalServerError, httpx.CodeInternal,
				"Đã xảy ra lỗi. Vui lòng thử lại.")
		},
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz(deps.DB))

	handler := openapi.HandlerWithOptions(strict, openapi.StdHTTPServerOptions{
		BaseRouter: mux,
		Middlewares: inExecutionOrder(
			httpx.RateLimit(limits, ratelimit.ClientIP(clientIPHeader)),
			httpx.WithRequestMeta(ratelimit.ClientIP(clientIPHeader)),
			WithRefreshCookie,
			httpx.RequireAuth(openRoutes, deps.verifyAccessToken),
			httpx.RequireRole,
			validate,
		),
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			httpx.WriteError(w, r, http.StatusBadRequest, httpx.CodeValidationFailed, err.Error())
		},
	})

	// Outermost first: an id and a log line exist even for a rejected preflight.
	return httpx.RequestID(httpx.Logging(logger)(httpx.CORS(allowedOrigins)(handler))), nil
}

func healthz(database DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := http.StatusOK
		body := map[string]string{"status": "ok", "database": "ok"}
		if database != nil {
			if err := database.Ping(r.Context()); err != nil {
				status = http.StatusServiceUnavailable
				body = map[string]string{"status": "degraded", "database": "unreachable"}
			}
		} else {
			body["database"] = "not configured"
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}
}

// inExecutionOrder reverses the middleware slice.
//
// oapi-codegen wraps them in order -- `handler = middleware(handler)` in a loop
// -- so the LAST entry ends up outermost and runs FIRST. Written literally,
// the list reads backwards from what happens, and the comments on it drift into
// describing an order that is not the real one. Reversing here lets the list
// above be read top-to-bottom as the sequence a request actually travels.
//
// router_test.go pins the direction, so an upstream change to how the generated
// wrapper applies middleware fails a test instead of silently inverting the
// chain.
// inExecutionOrder reverses the slice because oapi-codegen applies middleware
// last-first, so the argument order reads as execution order.
func inExecutionOrder(mw ...openapi.MiddlewareFunc) []openapi.MiddlewareFunc {
	out := make([]openapi.MiddlewareFunc, 0, len(mw))
	for i := len(mw) - 1; i >= 0; i-- {
		out = append(out, mw[i])
	}
	return out
}
