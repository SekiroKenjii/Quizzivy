package router_test

import (
	"net/http"
	"net/http/httptest"
	"quizzivy/internal/core/router"
	"strings"
	"testing"

	"quizzivy/gen/openapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/platform/ratelimit"
)

// §14 requires every public endpoint to be rate-limited, and §6.5 explains why:
// these are the only endpoints reachable without a session, and one of them
// takes a bearer secret.
//
// The contract is the source of truth, so adding a public operation to
// api/openapi.yaml is what creates the obligation -- nobody has to remember.

func TestEveryPublicOperationIsRateLimited(t *testing.T) {
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatalf("GetSwagger: %v", err)
	}
	if err := httpx.AssertPublicRoutesLimited(spec, router.RateLimits()); err != nil {
		t.Fatal(err)
	}
}

func TestTheAssertionActuallyFails(t *testing.T) {
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatalf("GetSwagger: %v", err)
	}

	empty := ratelimit.NewRegistry()
	err = httpx.AssertPublicRoutesLimited(spec, empty)
	if err == nil {
		t.Fatal("an empty registry must be reported as missing every public route")
	}
	for _, want := range []string{"POST /join/preview", "POST /auth/google", "POST /auth/login"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not name %q:\n%s", want, err)
		}
	}
}

func TestDroppingOneRouteIsCaught(t *testing.T) {
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatalf("GetSwagger: %v", err)
	}
	partial := ratelimit.NewRegistry()
	for _, pattern := range router.RateLimits().Patterns() {
		if pattern == "POST /join/preview" {
			continue
		}
		partial.Add(pattern, 100, ratelimit.PerMinute(10))
	}

	err = httpx.AssertPublicRoutesLimited(spec, partial)
	if err == nil {
		t.Fatal("removing /join/preview's limit must fail the assertion")
	}
	if !strings.Contains(err.Error(), "POST /join/preview") {
		t.Errorf("error should name the missing route:\n%s", err)
	}
}

func TestRegistryHasNoStaleEntries(t *testing.T) {
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatalf("GetSwagger: %v", err)
	}

	known := map[string]bool{}
	for path, item := range spec.Paths.Map() {
		for method := range item.Operations() {
			known[method+" "+path] = true
		}
	}

	for name, reg := range map[string]*ratelimit.Registry{
		"RateLimits":          router.RateLimits(),
		"PrincipalRateLimits": router.PrincipalRateLimits(),
	} {
		if len(reg.Patterns()) == 0 {
			t.Errorf("%s is empty: the walk is looking at the wrong thing", name)
		}
		for _, pattern := range reg.Patterns() {
			if !known[pattern] {
				t.Errorf("%s limits %q, which is not an operation in api/openapi.yaml", name, pattern)
			}
		}
	}
}

func TestPublicStatusFitsAClassroomBehindOneAddressAndNoMore(t *testing.T) {
	handler := newTestRouter(t, fakeDB{})
	get := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/public/status", nil)
		req.RemoteAddr = "203.0.113.30:5555"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	for i := 1; i <= 120; i++ {
		if rec := get(); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d was limited; 120 a minute are allowed", i)
		}
	}
	rec := get()
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("request 121: status = %d, Retry-After %q; want 429 with Retry-After", rec.Code, rec.Header().Get("Retry-After"))
	}
}
