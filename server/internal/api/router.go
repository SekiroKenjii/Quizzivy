package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"quizzivy/gen/openapi"
	"quizzivy/internal/httpx"
	"quizzivy/internal/ratelimit"
)

// RateLimits declares the policy for every public operation.
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

	reg.Add("POST /join/preview", capacity, ratelimit.PerMinute(10), ratelimit.PerHour(60))
	reg.Add("POST /auth/google", capacity, ratelimit.PerMinute(10), ratelimit.PerHour(60))
	reg.Add("POST /auth/login", capacity, ratelimit.PerMinute(10), ratelimit.PerHour(60))
	reg.Add("POST /auth/refresh", capacity, ratelimit.PerMinute(30), ratelimit.PerHour(200))
	reg.Add("POST /app/classes/join", capacity, ratelimit.PerMinute(10), ratelimit.PerHour(60))
	// Fire-and-forget beacon flush (§10.6). Limited generously: dropping these
	// costs integrity data, and integrity is observational.
	reg.Add("POST /app/attempts/{id}/events", capacity, ratelimit.PerMinute(120))

	return reg
}

// NewRouter builds the HTTP handler.
func NewRouter(deps Deps, logger *slog.Logger, allowedOrigins []string, clientIPHeader string) (http.Handler, error) {
	spec, err := openapi.GetSwagger()
	if err != nil {
		return nil, err
	}

	limits := RateLimits()
	// Refuses to start rather than shipping an unprotected public endpoint.
	if err := httpx.AssertPublicRoutesLimited(spec, limits); err != nil {
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
		// Applied per route, so r.Pattern is set and the limiter keys on the
		// OpenAPI path template rather than a concrete URL.
		Middlewares: []openapi.MiddlewareFunc{
			httpx.RateLimit(limits, ratelimit.ClientIP(clientIPHeader)),
		},
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
