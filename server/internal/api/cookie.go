package api

import (
	"context"
	"net/http"
	"time"
)

// refreshCookieName must match the `refreshCookie` security scheme in
// api/openapi.yaml. contract_test.go asserts it, because a rename on one side
// only would not fail to compile -- it would fail to log anyone in.
const refreshCookieName = "quizzivy_refresh"

type apiCtxKey int

const refreshTokenKey apiCtxKey = 0

// WithRefreshCookie lifts the refresh cookie into the request context.
//
// The generated strict handlers receive a typed request object, not the raw
// *http.Request, and a cookie declared as a security scheme is documentation
// rather than a parameter -- so the value has to be carried across some other
// way. Middleware keeps the cookie name in one package with the code that sets
// it, instead of teaching every handler how to find it.
func WithRefreshCookie(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(refreshCookieName)
		if err != nil || c.Value == "" {
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(
			context.WithValue(r.Context(), refreshTokenKey, c.Value)))
	})
}

// refreshTokenFromContext returns the presented token, or "" if there was none.
func refreshTokenFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(refreshTokenKey).(string); ok {
		return v
	}
	return ""
}

// refreshCookie builds the §5.2 cookie: httpOnly, Secure, SameSite=Lax,
// Path=/auth, and HOST-ONLY -- no Domain attribute, so only the API host ever
// receives it.
//
// SameSite=Lax is sufficient because app.quizzivy.com and api.quizzivy.com are
// the same SITE even though they are different origins. On genuinely cross-site
// hosts this cookie would never be sent and sessions would die silently
// (docs/plan/30-risks.md R-07).
func refreshCookie(token string, ttl time.Duration, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     "/auth",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	}
}

// clearRefreshCookie expires the cookie on logout.
//
// Every attribute that identifies the cookie -- name, Path, and the flags the
// browser matches on -- has to repeat exactly, or the browser stores a SECOND
// cookie instead of replacing the first and the user stays logged in.
func clearRefreshCookie(secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/auth",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1, // net/http renders this as Max-Age=0, expiring it now.
	}
}
