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
	// Enough for any of these request bodies; anything larger falls back to the
	// per-IP bucket rather than being buffered.
	const maxKeyBodyBytes = 8 * 1024

	// §6.5 requires BOTH buckets. Per-IP alone does not stop a distributed
	// probe of one code, and per-code alone does not stop one host walking the
	// code space. Neither is the realistic threat on its own -- forwarding is
	// (R-02) -- but 40 bits behind no limit is worth probing at scale.
	reg.Add("POST /join/preview", capacity, ratelimit.PerMinute(10), ratelimit.PerHour(60)).
		WithKey(ratelimit.JSONFieldKeyFunc("joinCode", maxKeyBodyBytes, join.Normalize), capacity, ratelimit.PerHour(30))
	// The contract's second bucket. With a joinCode this endpoint is the signup
	// path, so one code must not be usable to create accounts from a hundred
	// addresses. Requests without a joinCode are plain sign-ins and fall back
	// to the per-IP bucket alone, because JSONFieldKey yields no key for them.
	reg.Add("POST /auth/google", capacity, ratelimit.PerMinute(10), ratelimit.PerHour(60)).
		WithKey(ratelimit.JSONFieldKeyFunc("joinCode", maxKeyBodyBytes, join.Normalize), capacity, ratelimit.PerHour(30))
	// §6.5's second bucket: per-email as well as per-IP, so a distributed
	// attempt against ONE account is limited even though each source address
	// stays under its own allowance.
	reg.Add("POST /auth/login", capacity, ratelimit.PerMinute(10), ratelimit.PerHour(60)).
		WithKey(ratelimit.JSONFieldKey("email", maxKeyBodyBytes), capacity, ratelimit.PerHour(20))
	reg.Add("POST /auth/refresh", capacity, ratelimit.PerMinute(30), ratelimit.PerHour(200))
	// Authenticated by the refresh cookie rather than a bearer token, so it is
	// reachable by anyone and writes to the database on every call. Generous:
	// throttling logout would be a way to keep someone signed in.
	reg.Add("POST /auth/logout", capacity, ratelimit.PerMinute(30), ratelimit.PerHour(200))
	// Authenticated, but still a code-redemption endpoint: the contract asks
	// for the same two buckets, and a signed-in account is not a reason to let
	// one code be probed without limit.
	reg.Add("POST /app/classes/join", capacity, ratelimit.PerMinute(10), ratelimit.PerHour(60)).
		WithKey(ratelimit.JSONFieldKeyFunc("joinCode", maxKeyBodyBytes, join.Normalize), capacity, ratelimit.PerHour(30))
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
		// Applied per route, so r.Pattern is set and the limiter keys on the
		// OpenAPI path template rather than a concrete URL.
		Middlewares: inExecutionOrder(
			// Cheapest rejection first: a flood is turned away before it costs
			// a token verification or a schema walk.
			httpx.RateLimit(limits, ratelimit.ClientIP(clientIPHeader)),
			// Same address resolver as the limiter, so the value stored against
			// a session is the one that was limited rather than a second
			// notion of "the client" that disagrees behind a proxy.
			httpx.WithRequestMeta(ratelimit.ClientIP(clientIPHeader)),
			// The refresh cookie, which the generated strict handlers cannot
			// reach on their own. Absent on all but three routes; the
			// middleware is a no-op when there is no cookie to lift.
			WithRefreshCookie,
			// Authenticate before validating: an anonymous caller should be
			// told to log in, not handed a critique of their request body.
			// Fail-closed -- see httpx.RequireAuth.
			httpx.RequireAuth(openRoutes, deps.verifyAccessToken),
			// §3's route trees. Authentication says who you are; this says
			// whether the /admin tree is yours.
			httpx.RequireRole,
			// Last, so a 400 means "you are who you say you are, and this
			// request is still wrong".
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
func inExecutionOrder(mw ...openapi.MiddlewareFunc) []openapi.MiddlewareFunc {
	out := make([]openapi.MiddlewareFunc, 0, len(mw))
	for i := len(mw) - 1; i >= 0; i-- {
		out = append(out, mw[i])
	}
	return out
}
