package httpx

import (
	"context"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"quizzivy/internal/shared/access"
)

const principalKey ctxKey = 2

// Principal is the authenticated caller: the user a verified access token names
// and the session epoch it carries, and, once RequirePermission has resolved
// the user, what the user may do.
type Principal struct {
	UserID string
	Epoch  int
	Access access.Principal
}

// RequireAuth enforces bearer authentication on every generated route that the
// CONTRACT does not mark as open.
func RequireAuth(open map[string]struct{}, verify func(bearer string) (Principal, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, isOpen := open[r.Pattern]
			token, hasToken := bearerToken(r)
			if !hasToken {
				serveOrRefuse(next, w, r, isOpen)
				return
			}
			principal, err := verify(token)
			if err != nil {
				serveOrRefuse(next, w, r, isOpen)
				return
			}
			next.ServeHTTP(w, withPrincipal(r, principal))
		})
	}
}

func serveOrRefuse(next http.Handler, w http.ResponseWriter, r *http.Request, isOpen bool) {
	if isOpen {
		next.ServeHTTP(w, r)
		return
	}
	writeUnauthenticated(w, r)
}

func withPrincipal(r *http.Request, principal Principal) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), principalKey, principal))
}

// PrincipalFromContext returns the authenticated caller. The second result is
// false on an open route, where there may be no caller at all.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey).(Principal)
	return p, ok
}

func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", false
	}

	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}

func writeUnauthenticated(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="quizzivy"`)
	WriteError(w, r, http.StatusUnauthorized, CodeUnauthorized,
		TextFor(r, "Phiên đăng nhập không hợp lệ. Vui lòng đăng nhập lại.", "Your session is not valid. Please sign in again."))
}

// OpenRoutes lists the operations that do NOT require the named security
// scheme, keyed by the `METHOD /path` pattern the mux matches on.
func OpenRoutes(spec *openapi3.T, scheme string) map[string]struct{} {
	open := map[string]struct{}{}
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			if op == nil {
				continue
			}
			if !requiresScheme(op.Security, spec.Security, scheme) {
				open[method+" "+path] = struct{}{}
			}
		}
	}
	return open
}

func requiresScheme(opSecurity *openapi3.SecurityRequirements, global openapi3.SecurityRequirements, scheme string) bool {
	reqs := global
	if opSecurity != nil {
		reqs = *opSecurity
	}
	if len(reqs) == 0 {
		return false
	}
	for _, req := range reqs {
		if _, ok := req[scheme]; !ok {
			return false
		}
	}
	return true
}
