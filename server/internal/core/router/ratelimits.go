package router

import (
	classesdomain "quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/platform/ratelimit"
)

// RateLimits declares the policy for every public operation and for the in-app
// join, keyed by client address or by a field of the body. It is applied
// before authentication.
func RateLimits() *ratelimit.Registry {
	reg := ratelimit.NewRegistry()
	const capacity = 10_000
	const maxKeyBodyBytes = 8 * 1024
	joinCode := ratelimit.JSONFieldKeyFunc("joinCode", maxKeyBodyBytes, 9, classesdomain.JoinCodes.Normalize)
	email := ratelimit.JSONFieldKey("email", maxKeyBodyBytes, 254)
	reg.Add("POST /join/preview", capacity, ratelimit.PerMinute(120), ratelimit.PerHour(600)).
		WithKey("perCode", joinCode, capacity, ratelimit.PerHour(200))
	reg.Add("POST /auth/google", capacity, ratelimit.PerMinute(120), ratelimit.PerHour(600)).
		WithKey("perCode", joinCode, capacity, ratelimit.PerHour(200))
	reg.Add("POST /auth/login", capacity, ratelimit.PerMinute(120), ratelimit.PerHour(600)).
		WithKey("perAddressAndEmail", ratelimit.Compose(ratelimit.Address, email), capacity, ratelimit.PerMinute(10)).
		WithKey("perEmail", email, capacity, ratelimit.PerHour(20))
	reg.Add("POST /auth/refresh", capacity, ratelimit.PerMinute(120), ratelimit.PerHour(1200))
	reg.Add("POST /auth/logout", capacity, ratelimit.PerMinute(120), ratelimit.PerHour(1200))
	reg.Add("POST /app/classes/join", capacity, ratelimit.PerMinute(120), ratelimit.PerHour(600)).
		WithKey("perCode", joinCode, capacity, ratelimit.PerHour(200))
	reg.Add("POST /app/attempts/{id}/events", capacity, ratelimit.PerMinute(120))
	reg.Add("GET /public/status", capacity, ratelimit.PerMinute(120), ratelimit.PerHour(2000))

	return reg
}

// PrincipalRateLimits declares the limits counted per signed-in user, wherever
// the user connects from: each entry has no per-address bucket and one bucket,
// perActor, keyed by ratelimit.PrincipalKey. They are applied after the
// permission gate, so an entry may name only an operation that requires a
// token; New refuses any other. The authenticated operations that mint a
// credential are here.
func PrincipalRateLimits() *ratelimit.Registry {
	reg := ratelimit.NewRegistry()
	const capacity = 10_000
	perActor := func(pattern string, rules ...ratelimit.Rule) {
		reg.AddKeyed(pattern).WithKey("perActor", ratelimit.PrincipalKey, capacity, rules...)
	}
	perActor("POST /teacher/students/{id}/reset-password", ratelimit.PerMinute(5), ratelimit.PerHour(30))
	perActor("POST /teacher/students", ratelimit.PerMinute(30), ratelimit.PerHour(300))
	perActor("POST /teacher/classes/{id}/join-code", ratelimit.PerMinute(10), ratelimit.PerHour(60))
	perActor("GET /teacher/classes/{id}/join-code", ratelimit.PerMinute(60), ratelimit.PerHour(600))
	perActor("POST /admin/docs-session", ratelimit.PerMinute(5), ratelimit.PerHour(30))
	perActor("GET /teacher/assignments/results.csv", ratelimit.PerMinute(10), ratelimit.PerHour(60))
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
