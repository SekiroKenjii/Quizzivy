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
