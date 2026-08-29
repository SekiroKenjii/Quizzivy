package api

import (
	"strings"
	"testing"

	"quizzivy/gen/openapi"
	"quizzivy/internal/httpx"
	"quizzivy/internal/ratelimit"
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
	if err := httpx.AssertPublicRoutesLimited(spec, RateLimits()); err != nil {
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
	for _, pattern := range RateLimits().Patterns() {
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

	for _, pattern := range RateLimits().Patterns() {
		if !known[pattern] {
			t.Errorf("registry limits %q, which is not an operation in api/openapi.yaml", pattern)
		}
	}
}
