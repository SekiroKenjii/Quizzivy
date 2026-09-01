package httpx

import (
	"context"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

const principalKey ctxKey = 2

// Principal is the authenticated caller, as asserted by a verified access token.
type Principal struct {
	UserID string
	Role   string
}

// RequireAuth enforces bearer authentication on every generated route that the
// CONTRACT does not mark as open.
//
// It is deliberately fail-CLOSED. The obvious shape -- a set of protected
// routes, pass through anything else -- fails open: a route missing from the
// set serves data with no token and nothing looks wrong. Inverting it means a
// mistake shows up as a public endpoint returning 401, which someone notices in
// a minute, instead of a private one returning data, which nobody notices.
//
// `/healthz` is registered on the base mux rather than through the generated
// wrapper, so it never reaches this middleware.
func RequireAuth(open map[string]struct{}, verify func(bearer string) (Principal, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, isOpen := open[r.Pattern]; isOpen {
				next.ServeHTTP(w, r)
				return
			}

			token, ok := bearerToken(r)
			if !ok {
				writeUnauthenticated(w, r)
				return
			}
			principal, err := verify(token)
			if err != nil {
				writeUnauthenticated(w, r)
				return
			}

			next.ServeHTTP(w, r.WithContext(
				context.WithValue(r.Context(), principalKey, principal)))
		})
	}
}

// PrincipalFromContext returns the authenticated caller. The second result is
// false on an open route, where there may be no caller at all.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey).(Principal)
	return p, ok
}

// bearerToken pulls the credential out of an Authorization header.
func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", false
	}
	// The scheme is case-insensitive per RFC 7235; some clients send "bearer".
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
		"Phiên đăng nhập không hợp lệ. Vui lòng đăng nhập lại.")
}

// OpenRoutes lists the operations that do NOT require the named security
// scheme, keyed by the `METHOD /path` pattern the mux matches on.
//
// The contract is the source of truth: an operation inherits the document's
// top-level `security` unless it overrides it, so adding an endpoint without
// thinking about auth gets the safe default rather than none.
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
	for _, req := range reqs {
		if _, ok := req[scheme]; ok {
			return true
		}
	}
	return false
}

// RoleAdmin is the app.user_role value the /admin tree requires.
const RoleAdmin = "admin"

// AdminPathPrefix is spec §3's teacher route tree.
const AdminPathPrefix = "/admin/"

// RequireRole gates the /admin/ tree on the admin role.
//
// Driven by the path rather than a per-operation annotation, because the path is
// the contract's own structure and the cost of forgetting an annotation is a
// student reading every attempt in the school.
func RequireRole(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsAdminPattern(r.Pattern) {
			next.ServeHTTP(w, r)
			return
		}

		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			writeUnauthenticated(w, r)
			return
		}
		if principal.Role != RoleAdmin {
			WriteError(w, r, http.StatusForbidden, CodeForbidden,
				"Bạn không có quyền truy cập chức năng này.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// IsAdminPattern reports whether a mux pattern -- "GET /admin/classes/{id}" --
// addresses the admin tree.
func IsAdminPattern(pattern string) bool {
	_, path, found := strings.Cut(pattern, " ")
	if !found {
		path = pattern
	}
	return strings.HasPrefix(path, AdminPathPrefix)
}
