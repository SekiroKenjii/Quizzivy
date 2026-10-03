package http

import (
	"context"
	"net/http"
	"time"
)

const refreshCookieName = "quizzivy_refresh"

type apiCtxKey int

const refreshTokenKey apiCtxKey = 0

// WithRefreshCookie lifts the refresh cookie into the request context.
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

func refreshTokenFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(refreshTokenKey).(string); ok {
		return v
	}
	return ""
}

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

type clearedSession []*http.Cookie

func (c clearedSession) VisitLogoutResponse(w http.ResponseWriter) error {
	for _, cookie := range c {
		w.Header().Add("Set-Cookie", cookie.String())
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// ClearSessionOnRefusedLogout adds the refresh and docs cookie clears to the
// answer of a POST /auth/logout the maintenance gate refuses, when the
// request presented a refresh cookie, so the device is signed out although
// the refresh family is not revoked.
func (h Identity) ClearSessionOnRefusedLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/auth/logout" {
		return
	}
	if c, err := r.Cookie(refreshCookieName); err != nil || c.Value == "" {
		return
	}
	for _, cookie := range (clearedSession{clearRefreshCookie(h.cookieSecure), clearDocsCookie()}) {
		w.Header().Add("Set-Cookie", cookie.String())
	}
}

func clearRefreshCookie(secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/auth",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	}
}
