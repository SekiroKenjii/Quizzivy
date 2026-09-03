package httpx

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
)

// CORS for the SPA at app.quizzivy.com calling the API at api.quizzivy.com.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make([]string, 0, len(allowedOrigins))
	for _, o := range allowedOrigins {
		if trimmed := strings.TrimSpace(o); trimmed != "" && trimmed != "*" {
			allowed = append(allowed, trimmed)
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			permitted := origin != "" && slices.Contains(allowed, origin)

			w.Header().Add("Vary", "Origin")
			if permitted {
				allowOrigin(w, origin)
			}

			if isPreflight(r) {
				writePreflight(w, permitted)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isPreflight(r *http.Request) bool {
	return r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
}

func allowOrigin(w http.ResponseWriter, origin string) {
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", origin)
	h.Set("Access-Control-Allow-Credentials", "true")
	h.Set("Access-Control-Expose-Headers", "X-Request-Id, Retry-After")
}

func writePreflight(w http.ResponseWriter, permitted bool) {
	const maxAge = 600

	h := w.Header()
	h.Add("Vary", "Access-Control-Request-Method")
	h.Add("Vary", "Access-Control-Request-Headers")
	if permitted {
		h.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")
		h.Set("Access-Control-Max-Age", strconv.Itoa(maxAge))
	}
	w.WriteHeader(http.StatusNoContent)
}
