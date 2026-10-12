package httpx

import (
	"net/http"
	"strconv"
	"time"

	"quizzivy/internal/platform/ratelimit"
)

// RateLimit applies the policy registered for the matched route: the
// per-address bucket, then each keyed bucket in order. It records the client
// address on the request, so a composite key can read it through
// ratelimit.Address, and passes that same request on, since a body key
// restores the body on it. Each onRefuse runs on a refused request before the
// 429 is written, so a caller can add headers to that answer.
func RateLimit(reg *ratelimit.Registry, clientIP ratelimit.KeyFunc, onRefuse ...func(http.ResponseWriter, *http.Request)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = ratelimit.WithAddress(r, clientIP(r))
			if retry, limited := exceeded(reg, r); limited {
				for _, refuse := range onRefuse {
					refuse(w, r)
				}
				writeRateLimited(w, r, retry.Seconds())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// PrincipalRateLimit applies the policy registered for the matched route to
// the user RequirePermission resolved, so it belongs directly after that
// middleware. It records the resolved principal's user id on the request,
// where a bucket keyed by ratelimit.PrincipalKey reads it, and refuses as
// RateLimit does: the same 429, Retry-After and RATE_LIMITED. A request with
// no resolved principal passes untouched and spends no budget.
func PrincipalRateLimit(reg *ratelimit.Registry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			caller, ok := PrincipalFromContext(r.Context())
			if !ok || caller.Access.UserID == "" {
				next.ServeHTTP(w, r)
				return
			}
			r = ratelimit.WithPrincipal(r, caller.Access.UserID)
			if retry, limited := exceeded(reg, r); limited {
				writeRateLimited(w, r, retry.Seconds())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func exceeded(reg *ratelimit.Registry, r *http.Request) (time.Duration, bool) {
	route, ok := reg.Lookup(r.Pattern)
	if !ok {
		return 0, false
	}
	if route.PerIP != nil {
		if allowed, retry := route.PerIP.Allow(ratelimit.Address(r)); !allowed {
			return retry, true
		}
	}
	for _, bucket := range route.Keyed {
		key := bucket.Key(r)
		if key == "" {
			continue
		}
		if allowed, retry := bucket.Limiter.Allow(key); !allowed {
			return retry, true
		}
	}
	return 0, false
}

func writeRateLimited(w http.ResponseWriter, r *http.Request, seconds float64) {
	retry := int(seconds)
	if retry < 1 {
		retry = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(retry))
	WriteError(w, r, http.StatusTooManyRequests, CodeRateLimited,
		TextFor(r, "Bạn thao tác quá nhanh. Vui lòng thử lại sau.", "You are going too fast. Please try again later."))
}
