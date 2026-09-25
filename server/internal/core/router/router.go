// Package router assembles the HTTP surface: the generated strict handler over every module transport, the middleware in execution order, the health and docs endpoints, and the rate-limit policy.
package router

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"quizzivy/gen/openapi"
	identityhttp "quizzivy/internal/modules/identity/http"
	"quizzivy/internal/platform/apidocs"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/platform/ratelimit"
)

// New builds the HTTP handler.
func New(deps Deps, logger *slog.Logger, allowedOrigins []string, clientIPHeader string) (http.Handler, error) {
	spec, err := openapi.GetSpec()
	if err != nil {
		return nil, err
	}

	limits := RateLimits()

	if err := httpx.AssertPublicRoutesLimited(spec, limits); err != nil {
		return nil, err
	}

	openRoutes := httpx.OpenRoutes(spec, "bearerAuth")

	validate, err := httpx.ValidateRequests(spec)
	if err != nil {
		return nil, err
	}

	server := &Server{Imports: deps.Modules.Imports, Dashboard: deps.Modules.Dashboard, Classes: deps.Modules.Classes, Identity: deps.Modules.Identity, Questions: deps.Modules.Questions, Media: deps.Modules.Media, Tests: deps.Modules.Tests, Assignments: deps.Modules.Assignments, Attempts: deps.Modules.Attempts, Deps: deps, Logger: logger}
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
	mux.Handle("GET /docs", apidocs.Reference("/docs/openapi.json"))
	mux.Handle("GET /docs/openapi.json", apidocs.Spec(openapi.GetSpecJSON))

	handler := openapi.HandlerWithOptions(strict, openapi.StdHTTPServerOptions{
		BaseRouter: mux,
		Middlewares: inExecutionOrder(
			httpx.RateLimit(limits, ratelimit.ClientIP(clientIPHeader)),
			httpx.WithRequestMeta(ratelimit.ClientIP(clientIPHeader)),
			identityhttp.WithRefreshCookie,
			httpx.RequireAuth(openRoutes, deps.verifyAccessToken),
			httpx.RequireRole,
			httpx.LimitRequestBody(httpx.StreamingBodyRoutes(spec), 1<<20, httpx.RequestBodyLimits(spec)),
			validate,
		),
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			httpx.WriteError(w, r, http.StatusBadRequest, httpx.CodeValidationFailed, err.Error())
		},
	})

	return httpx.RequestID(httpx.Logging(logger)(httpx.SecurityHeaders(httpx.CORS(allowedOrigins)(handler)))), nil
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

func inExecutionOrder(mw ...openapi.MiddlewareFunc) []openapi.MiddlewareFunc {
	out := make([]openapi.MiddlewareFunc, 0, len(mw))
	for i := len(mw) - 1; i >= 0; i-- {
		out = append(out, mw[i])
	}
	return out
}
