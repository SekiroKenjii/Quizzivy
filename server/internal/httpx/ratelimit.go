package httpx

import (
	"net/http"
	"strconv"

	"quizzivy/internal/ratelimit"
)

// RateLimit applies the policy registered for the matched route.
//
// It runs as a per-route middleware so `r.Pattern` is already populated by the
// mux — which means the key is the OpenAPI path template, not a concrete URL,
// and `/admin/tests/{id}` shares one bucket rather than creating one per test.
func RateLimit(reg *ratelimit.Registry, clientIP ratelimit.KeyFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route, ok := reg.Lookup(r.Pattern)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			if route.PerIP != nil {
				if allowed, retry := route.PerIP.Allow(clientIP(r)); !allowed {
					writeRateLimited(w, r, retry.Seconds())
					return
				}
			}

			if route.PerKey != nil && route.Key != nil {
				if key := route.Key(r); key != "" {
					if allowed, retry := route.PerKey.Allow(key); !allowed {
						writeRateLimited(w, r, retry.Seconds())
						return
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeRateLimited(w http.ResponseWriter, r *http.Request, seconds float64) {
	retry := int(seconds)
	if retry < 1 {
		retry = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(retry))
	WriteError(w, r, http.StatusTooManyRequests, CodeRateLimited, "Bạn thao tác quá nhanh. Vui lòng thử lại sau.")
}
