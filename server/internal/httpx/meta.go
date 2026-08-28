package httpx

import (
	"context"
	"net/http"
)

const metaKey ctxKey = 1

// RequestMeta is the request context worth recording alongside a session or an
// audit row: §6.5 requires every enrolment to log ip and user agent, and §13.5
// stores both against a refresh token so a suspicious session is identifiable.
type RequestMeta struct {
	IP        string
	UserAgent string
}

// WithRequestMeta records the client address and user agent.
//
// The address comes from the same resolver the rate limiter uses, so the value
// stored against a session is the same one that was limited -- rather than two
// notions of "the client" that disagree behind a proxy.
func WithRequestMeta(clientIP func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			meta := RequestMeta{
				IP:        clientIP(r),
				UserAgent: r.UserAgent(),
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), metaKey, meta)))
		})
	}
}

func RequestMetaFromContext(ctx context.Context) RequestMeta {
	if m, ok := ctx.Value(metaKey).(RequestMeta); ok {
		return m
	}
	return RequestMeta{}
}
