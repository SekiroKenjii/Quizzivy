package router

import (
	classesdomain "quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/platform/ratelimit"
)

// RateLimits declares the policy for every public operation, plus the
// authenticated operations that mint a credential.
func RateLimits() *ratelimit.Registry {
	reg := ratelimit.NewRegistry()
	const capacity = 10_000
	const maxKeyBodyBytes = 8 * 1024
	reg.Add("POST /join/preview", capacity, ratelimit.PerMinute(10), ratelimit.PerHour(60)).
		WithKey(ratelimit.JSONFieldKeyFunc("joinCode", maxKeyBodyBytes, classesdomain.JoinCodes.Normalize), capacity, ratelimit.PerHour(30))
	reg.Add("POST /auth/google", capacity, ratelimit.PerMinute(10), ratelimit.PerHour(60)).
		WithKey(ratelimit.JSONFieldKeyFunc("joinCode", maxKeyBodyBytes, classesdomain.JoinCodes.Normalize), capacity, ratelimit.PerHour(30))
	reg.Add("POST /auth/login", capacity, ratelimit.PerMinute(10), ratelimit.PerHour(60)).
		WithKey(ratelimit.JSONFieldKey("email", maxKeyBodyBytes), capacity, ratelimit.PerHour(20))
	reg.Add("POST /auth/refresh", capacity, ratelimit.PerMinute(30), ratelimit.PerHour(200))
	reg.Add("POST /auth/logout", capacity, ratelimit.PerMinute(30), ratelimit.PerHour(200))
	reg.Add("POST /app/classes/join", capacity, ratelimit.PerMinute(10), ratelimit.PerHour(60)).
		WithKey(ratelimit.JSONFieldKeyFunc("joinCode", maxKeyBodyBytes, classesdomain.JoinCodes.Normalize), capacity, ratelimit.PerHour(30))
	reg.Add("POST /app/attempts/{id}/events", capacity, ratelimit.PerMinute(120))

	reg.Add("POST /admin/students/{id}/reset-password", capacity,
		ratelimit.PerMinute(5), ratelimit.PerHour(30))
	reg.Add("POST /admin/docs-session", capacity, ratelimit.PerMinute(5), ratelimit.PerHour(30))

	return reg
}

// ServiceRateLimits declares the policy for the routes served beside the
// contract, which the generated middleware chain never sees.
func ServiceRateLimits() *ratelimit.Registry {
	reg := ratelimit.NewRegistry()
	const capacity = 10_000
	reg.Add("GET /livez", capacity, ratelimit.PerMinute(30), ratelimit.PerHour(900))
	reg.Add("GET /healthz", capacity, ratelimit.PerMinute(10), ratelimit.PerHour(120))
	reg.Add("GET /docs", capacity, ratelimit.PerMinute(20), ratelimit.PerHour(200))
	reg.Add("GET /docs/openapi.json", capacity, ratelimit.PerMinute(20), ratelimit.PerHour(200))
	return reg
}
