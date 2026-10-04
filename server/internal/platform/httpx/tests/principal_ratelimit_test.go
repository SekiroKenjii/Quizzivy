package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/platform/ratelimit"
	"quizzivy/internal/shared/access"
)

func perActor(pattern string, perMinute int) *ratelimit.Registry {
	reg := ratelimit.NewRegistry()
	reg.AddKeyed(pattern).WithKey("perActor", ratelimit.PrincipalKey, 100, ratelimit.PerMinute(perMinute))
	return reg
}

func callFrom(h http.Handler, pattern, token, address string) *httptest.ResponseRecorder {
	method, path, _ := strings.Cut(pattern, " ")
	req := httptest.NewRequest(method, path, nil)
	req.Pattern = pattern
	req.RemoteAddr = address + ":1234"
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestThePrincipalLimiterCountsPerUserWhateverTheAddress(t *testing.T) {
	reached := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached++
		if got := ratelimit.PrincipalKey(r); got != "grader" && got != "auditor" {
			t.Errorf("the handler's request carries principal key %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	h := gateBefore(principals, httpx.PrincipalRateLimit(perActor("GET /self", 2))(next))

	for i, address := range []string{"198.51.100.1", "198.51.100.2"} {
		if rec := callFrom(h, "GET /self", "grader@0", address); rec.Code != http.StatusNoContent {
			t.Fatalf("request %d of one user: %d, want it through", i+1, rec.Code)
		}
	}
	refused := callFrom(h, "GET /self", "grader@0", "198.51.100.3")
	if refused.Code != http.StatusTooManyRequests {
		t.Fatalf("the third request of one user, from a third address: %d, want 429", refused.Code)
	}
	if seconds, err := strconv.Atoi(refused.Header().Get("Retry-After")); err != nil || seconds < 1 {
		t.Errorf("Retry-After = %q, want whole seconds of at least 1", refused.Header().Get("Retry-After"))
	}
	if reached != 2 {
		t.Errorf("the handler ran %d times, want 2: a refused request must not reach it", reached)
	}
	if rec := callFrom(h, "GET /self", "auditor@0", "198.51.100.1"); rec.Code != http.StatusNoContent {
		t.Errorf("another user behind the first address: %d, want it through", rec.Code)
	}
}

func TestThePrincipalLimiterLeavesUnregisteredRoutesAlone(t *testing.T) {
	h := gateBefore(principals, httpx.PrincipalRateLimit(perActor("GET /self", 1))(letThrough()))
	for i := 1; i <= 3; i++ {
		if got := call(h, "GET /review", "grader@0"); got != http.StatusNoContent {
			t.Fatalf("request %d to a route with no entry: %d, want it through", i, got)
		}
	}
	if got := call(h, "GET /self", "grader@0"); got != http.StatusNoContent {
		t.Errorf("the first request to the limited route: %d, want it through: other routes spent its budget", got)
	}
}

func TestThePrincipalLimiterPassesWhereNoPrincipalWasResolved(t *testing.T) {
	h := gateBefore(principals, httpx.PrincipalRateLimit(perActor("POST /open", 1))(letThrough()))
	for _, token := range []string{"", "student@0", "student@0", "student@0"} {
		if got := call(h, "POST /open", token); got != http.StatusNoContent {
			t.Errorf("an open route with token %q: %d, want it through: no principal is resolved there", token, got)
		}
	}
	bare := httpx.PrincipalRateLimit(perActor("GET /self", 1))(letThrough())
	for i := 1; i <= 3; i++ {
		if got := call(bare, "GET /self", "student@0"); got != http.StatusNoContent {
			t.Errorf("request %d with no gate before the limiter: %d, want it through", i, got)
		}
	}
}

func TestThePrincipalLimiterKeysOnTheResolvedUserNotTheTokensSubject(t *testing.T) {
	sameUser := resolver{
		"laptop": {UserID: "lan", Permissions: access.NewSet(access.TeachingGrading)},
		"phone":  {UserID: "lan", Permissions: access.NewSet(access.TeachingGrading)},
		"minh":   {UserID: "minh", Permissions: access.NewSet(access.TeachingGrading)},
	}
	h := gateBefore(sameUser, httpx.PrincipalRateLimit(perActor("GET /review", 1))(letThrough()))

	if got := call(h, "GET /review", "laptop@0"); got != http.StatusNoContent {
		t.Fatalf("the first request: %d, want it through", got)
	}
	if got := call(h, "GET /review", "phone@0"); got != http.StatusTooManyRequests {
		t.Errorf("a second token that resolves to the same user: %d, want 429 from the shared budget", got)
	}
	if got := call(h, "GET /review", "minh@0"); got != http.StatusNoContent {
		t.Errorf("another user: %d, want it through", got)
	}
}

func TestARefusalByTheGateSpendsNoPrincipalBudget(t *testing.T) {
	moved := resolver{"lan": {UserID: "lan", Permissions: access.NewSet(access.LearningTakeTests)}}
	h := gateBefore(moved, httpx.PrincipalRateLimit(perActor("GET /review", 1))(letThrough()))

	for i := 1; i <= 3; i++ {
		if got := call(h, "GET /review", "lan@0"); got != http.StatusForbidden {
			t.Fatalf("request %d without the permission: %d, want 403 every time", i, got)
		}
	}
	moved["lan"] = access.Principal{UserID: "lan", Permissions: access.NewSet(access.TeachingGrading)}
	if got := call(h, "GET /review", "lan@0"); got != http.StatusNoContent {
		t.Errorf("the first request once the permission is granted: %d, want it through: the refusals spent the budget", got)
	}
	if got := call(h, "GET /review", "lan@0"); got != http.StatusTooManyRequests {
		t.Errorf("the second permitted request: %d, want 429", got)
	}
}
