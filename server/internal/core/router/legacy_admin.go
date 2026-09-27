package router

import (
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"strings"

	"quizzivy/internal/platform/httpx"
)

// LegacyAdminPaths returns the v0.7.0 /admin operations that R2 moved, each
// keyed by its old route pattern ("GET /admin/tests/{id}") and paired with the
// path pattern that serves it now ("/teacher/tests/{id}"). The router answers
// the old patterns through the new ones for one release; R3 removes them.
func LegacyAdminPaths() map[string]string {
	return maps.Clone(legacyAdminPaths)
}

var legacyAdminPaths = legacyTargets()

func legacyTargets() map[string]string {
	out := make(map[string]string, len(legacyAdminPatterns))
	for _, pattern := range legacyAdminPatterns {
		_, path, _ := strings.Cut(pattern, " ")
		out[pattern] = "/teacher/" + strings.TrimPrefix(path, "/admin/")
	}
	out["DELETE /admin/students/{id}"] = "/admin/users/{id}"
	return out
}

func legacyAdmin(logger *slog.Logger) func(http.Handler) http.Handler {
	aliases := http.NewServeMux()
	for pattern := range legacyAdminPaths {
		aliases.Handle(pattern, http.NotFoundHandler())
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, "/admin/") {
				next.ServeHTTP(w, r)
				return
			}
			_, pattern := aliases.Handler(r)
			target, ok := legacyAdminPaths[pattern]
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			rewritten, ok := rewriteLegacy(r, pattern, target)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			logger.Info("legacy_admin_path", "method", r.Method, "pattern", pattern,
				"request_id", httpx.RequestIDFromContext(r.Context()))
			next.ServeHTTP(w, rewritten)
		})
	}
}

func rewriteLegacy(r *http.Request, pattern, target string) (*http.Request, bool) {
	_, oldPath, _ := strings.Cut(pattern, " ")
	oldSegments := strings.Split(oldPath, "/")
	actual := strings.Split(r.URL.EscapedPath(), "/")
	if len(actual) != len(oldSegments) {
		return nil, false
	}
	wildcards := map[string]string{}
	for i, segment := range oldSegments {
		if strings.HasPrefix(segment, "{") {
			wildcards[segment] = actual[i]
		}
	}
	segments := strings.Split(target, "/")
	for i, segment := range segments {
		if value, ok := wildcards[segment]; ok {
			segments[i] = value
		}
	}
	parsed, err := url.Parse(strings.Join(segments, "/"))
	if err != nil {
		return nil, false
	}
	rewritten := r.Clone(r.Context())
	rewritten.URL.Path = parsed.Path
	rewritten.URL.RawPath = parsed.RawPath
	return rewritten, true
}
