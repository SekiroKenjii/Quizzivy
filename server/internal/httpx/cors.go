package httpx

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
)

// CORS for the SPA at app.quizzivy.com calling the API at api.quizzivy.com.
//
// Those are different ORIGINS but the same SITE, which is what lets the
// SameSite=Lax refresh cookie work at all (docs/plan/00-overview.md §4.1). This
// API therefore always sends credentials, and that forces two rules:
//
//   - The allowlist is exact. `*` is illegal with credentials — browsers reject
//     the combination outright — so a wildcard here does not loosen security,
//     it silently breaks every authenticated request.
//   - `Vary: Origin` is mandatory. Without it a shared cache can serve one
//     origin's Access-Control-Allow-Origin header to another.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make([]string, 0, len(allowedOrigins))
	for _, o := range allowedOrigins {
		if trimmed := strings.TrimSpace(o); trimmed != "" && trimmed != "*" {
			allowed = append(allowed, trimmed)
		}
	}

	const maxAge = 600

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Always vary, even when the origin is not allowed: the response
			// still differs by origin, and a cache must not conflate them.
			w.Header().Add("Vary", "Origin")

			if origin != "" && slices.Contains(allowed, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Expose-Headers", "X-Request-Id, Retry-After")
			}

			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.Header().Add("Vary", "Access-Control-Request-Method")
				w.Header().Add("Vary", "Access-Control-Request-Headers")
				if origin != "" && slices.Contains(allowed, origin) {
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")
					w.Header().Set("Access-Control-Max-Age", strconv.Itoa(maxAge))
				}
				// A disallowed origin gets 204 with no allow headers, which is
				// what makes the browser block the real request.
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
