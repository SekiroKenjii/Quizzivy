package httpx

import (
	"net/http"
	"strconv"
	"time"

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
			if retry, limited := exceeded(reg, clientIP, r); limited {
				writeRateLimited(w, r, retry.Seconds())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// exceeded reports whether the request has spent either of its route's buckets,
// and how long until the spent one refills.
func exceeded(reg *ratelimit.Registry, clientIP ratelimit.KeyFunc, r *http.Request) (time.Duration, bool) {
	route, ok := reg.Lookup(r.Pattern)
	if !ok {
		return 0, false
	}
	if route.PerIP != nil {
		if allowed, retry := route.PerIP.Allow(clientIP(r)); !allowed {
			return retry, true
		}
	}
	if route.PerKey == nil || route.Key == nil {
		return 0, false
	}
	key := route.Key(r)
	if key == "" {
		return 0, false
	}
	allowed, retry := route.PerKey.Allow(key)
	return retry, !allowed
}

func writeRateLimited(w http.ResponseWriter, r *http.Request, seconds float64) {
	retry := int(seconds)
	if retry < 1 {
		retry = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(retry))
	WriteError(w, r, http.StatusTooManyRequests, CodeRateLimited, "Bạn thao tác quá nhanh. Vui lòng thử lại sau.")
}
