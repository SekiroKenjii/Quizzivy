// Package router assembles the HTTP surface: the generated strict handler over every module transport, the middleware in execution order, the liveness, health and docs endpoints, and the rate-limit policy.
package router

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"reflect"

	"quizzivy/gen/openapi"
	identityhttp "quizzivy/internal/modules/identity/http"
	"quizzivy/internal/platform/apidocs"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/platform/ratelimit"
	"quizzivy/internal/shared/access"
)

// New builds the HTTP handler.
func New(deps Deps, logger *slog.Logger, allowedOrigins []string, clientIPHeader string) (http.Handler, error) {
	spec, err := openapi.GetSpec()
	if err != nil {
		return nil, err
	}

	bodyLimits, err := httpx.RequestBodyLimits(spec)
	if err != nil {
		return nil, err
	}

	limits := RateLimits()
	principalLimits := PrincipalRateLimits()

	if err := httpx.AssertPublicRoutesLimited(spec, limits); err != nil {
		return nil, err
	}
	if err := httpx.AssertPrincipalRoutesGated(spec, "bearerAuth", principalLimits); err != nil {
		return nil, err
	}
	if deps.Principals == nil {
		return nil, errors.New("router: no principal resolver; every gated operation would be refused")
	}
	requirements, err := httpx.PermissionRequirements(spec, "bearerAuth")
	if err != nil {
		return nil, err
	}

	openRoutes := httpx.OpenRoutes(spec, "bearerAuth")

	validate, err := httpx.ValidateRequests(spec)
	if err != nil {
		return nil, err
	}

	server := &Server{Imports: deps.Modules.Imports, Dashboard: deps.Modules.Dashboard, Classes: deps.Modules.Classes, Identity: deps.Modules.Identity, Questions: deps.Modules.Questions, Media: deps.Modules.Media, Tests: deps.Modules.Tests, Assignments: deps.Modules.Assignments, Attempts: deps.Modules.Attempts, Availability: deps.Modules.Availability, Notifications: deps.Modules.Notifications, Deps: deps, Logger: logger}
	strict := openapi.NewStrictHandlerWithOptions(server, nil, openapi.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, _ error) {
			httpx.WriteMalformedBody(w, r)
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			if errors.Is(err, httpx.ErrNotImplemented) {
				httpx.WriteError(w, r, http.StatusNotImplemented, httpx.CodeInternal,
					httpx.TextFor(r, "Chức năng này chưa được xây dựng.", "This feature has not been built yet."))
				return
			}
			logger.Error("handler", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
			httpx.WriteError(w, r, http.StatusInternalServerError, httpx.CodeInternal,
				httpx.TextFor(r, "Đã xảy ra lỗi. Vui lòng thử lại.", "Something went wrong. Try again."))
		},
	})

	limited := httpx.RateLimit(ServiceRateLimits(), ratelimit.ClientIP(clientIPHeader))
	mux := http.NewServeMux()
	mux.Handle("GET /livez", limited(http.HandlerFunc(livez)))
	mux.Handle("GET /healthz", limited(healthz(deps.DB)))
	docs := limited
	if !deps.DocsPublic {
		gate := identityhttp.RequireDocsSession(deps.Docs, deps.Principals)
		docs = func(next http.Handler) http.Handler { return limited(gate(next)) }
	}
	mux.Handle("GET /docs", docs(apidocs.Reference("/docs/openapi.json")))
	mux.Handle("GET /docs/openapi.json", docs(apidocs.Spec(openapi.GetSpecJSON)))

	handler := openapi.HandlerWithOptions(strict, openapi.StdHTTPServerOptions{
		BaseRouter: mux,
		Middlewares: inExecutionOrder(
			httpx.RateLimit(limits, ratelimit.ClientIP(clientIPHeader), deps.Modules.Identity.ClearSessionOnRefusedLogout),
			httpx.WithRequestMeta(ratelimit.ClientIP(clientIPHeader)),
			identityhttp.WithRefreshCookie,
			identityhttp.WithGeoLabel(clientIPHeader),
			httpx.RequireAuth(openRoutes, deps.verifyAccessToken),
			httpx.RequirePermission(requirements, deps.Principals),
			httpx.PrincipalRateLimit(principalLimits),
			httpx.LimitRequestBody(httpx.StreamingBodyRoutes(spec), 1<<20, bodyLimits),
			validate,
		),
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			httpx.WriteMalformedParameter(w, r, parameterName(err))
		},
	})

	gated := routedOnly(mux, httpx.Maintenance(deps.Maintenance, deps.Modules.Identity.ClearSessionOnRefusedLogout)(servedMethodOnly(mux, handler, requirements, openRoutes)))
	return httpx.RequestID(httpx.Logging(logger)(httpx.SecurityHeaders(httpx.CORS(allowedOrigins)(legacyAdmin(logger)(gated))))), nil
}

func parameterName(err error) string {
	var (
		format    *openapi.InvalidParamFormatError
		required  *openapi.RequiredParamError
		header    *openapi.RequiredHeaderError
		unmarshal *openapi.UnmarshalingParamError
		tooMany   *openapi.TooManyValuesForParamError
		cookie    *openapi.UnescapedCookieParamError
	)
	switch {
	case errors.As(err, &format):
		return format.ParamName
	case errors.As(err, &required):
		return required.ParamName
	case errors.As(err, &header):
		return header.ParamName
	case errors.As(err, &unmarshal):
		return unmarshal.ParamName
	case errors.As(err, &tooMany):
		return tooMany.ParamName
	case errors.As(err, &cookie):
		return cookie.ParamName
	}
	return ""
}

var probeMethods = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}

var muxRedirect = reflect.TypeOf(http.RedirectHandler("/", http.StatusTemporaryRedirect))

func routedOnly(mux *http.ServeMux, gated http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if routed(mux, r) {
			gated.ServeHTTP(w, r)
			return
		}
		if own, _ := mux.Handler(r); redirects(own) {
			mux.ServeHTTP(w, r)
			return
		}
		httpx.WriteNotFound(w, r)
	})
}

func routed(mux *http.ServeMux, r *http.Request) bool {
	if _, pattern := mux.Handler(r); pattern != "" {
		return true
	}
	probe := r.Clone(r.Context())
	for _, method := range probeMethods {
		probe.Method = method
		if _, pattern := mux.Handler(probe); pattern != "" {
			return true
		}
	}
	return false
}

func redirects(own http.Handler) bool {
	return reflect.TypeOf(own) == muxRedirect
}

func servedMethodOnly(mux *http.ServeMux, next http.Handler, needsToken map[string]access.Requirement, open map[string]struct{}) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		own, pattern := mux.Handler(r)
		head := r.Method == http.MethodHead
		_, tokenGated := needsToken[pattern]
		_, isOpen := open[pattern]
		switch {
		case redirects(own):
			next.ServeHTTP(w, r)
		case pattern == "", head && tokenGated:
			httpx.WriteMethodNotAllowed(w, r, servedMethods(mux, r, needsToken))
		case head && isOpen:
			next.ServeHTTP(bodiless{w}, asGet(r))
		default:
			next.ServeHTTP(w, r)
		}
	})
}

func servedMethods(mux *http.ServeMux, r *http.Request, needsToken map[string]access.Requirement) []string {
	var methods []string
	probe := r.Clone(r.Context())
	for _, method := range probeMethods {
		probe.Method = method
		_, pattern := mux.Handler(probe)
		if pattern == "" {
			continue
		}
		methods = append(methods, method)
		if _, tokenGated := needsToken[pattern]; method == http.MethodGet && !tokenGated {
			methods = append(methods, http.MethodHead)
		}
	}
	return methods
}

func asGet(r *http.Request) *http.Request {
	get := r.Clone(r.Context())
	get.Method = http.MethodGet
	return get
}

type bodiless struct{ http.ResponseWriter }

func (bodiless) Write(p []byte) (int, error) { return len(p), nil }

func (b bodiless) Unwrap() http.ResponseWriter { return b.ResponseWriter }

type health struct {
	Status   string `json:"status"`
	Database string `json:"database,omitempty"`
}

func livez(w http.ResponseWriter, _ *http.Request) {
	writeHealth(w, http.StatusOK, health{Status: "ok"})
}

func healthz(database DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case database == nil:
			writeHealth(w, http.StatusOK, health{Status: "ok", Database: "not configured"})
		case database.Ping(r.Context()) != nil:
			writeHealth(w, http.StatusServiceUnavailable, health{Status: "degraded", Database: "unreachable"})
		default:
			writeHealth(w, http.StatusOK, health{Status: "ok", Database: "ok"})
		}
	}
}

func writeHealth(w http.ResponseWriter, status int, body health) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func inExecutionOrder(mw ...openapi.MiddlewareFunc) []openapi.MiddlewareFunc {
	out := make([]openapi.MiddlewareFunc, 0, len(mw))
	for i := len(mw) - 1; i >= 0; i-- {
		out = append(out, mw[i])
	}
	return out
}
