package ratelimit_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"quizzivy/internal/platform/ratelimit"
)

func TestPrincipalKeyIsTheRecordedUserAndEmptyWithoutOne(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/teacher/students", nil)
	if got := ratelimit.PrincipalKey(r); got != "" {
		t.Errorf("an unrecorded request has principal key %q, want the bucket skipped", got)
	}
	if got := ratelimit.PrincipalKey(ratelimit.WithAddress(r, "203.0.113.4")); got != "" {
		t.Errorf("an address alone gave principal key %q, want the bucket skipped", got)
	}
	recorded := ratelimit.WithPrincipal(ratelimit.WithAddress(r, "203.0.113.4"), "01935000-0000-7000-8000-0000000000c3")
	if got := ratelimit.PrincipalKey(recorded); got != "01935000-0000-7000-8000-0000000000c3" {
		t.Errorf("the recorded user read back as %q", got)
	}
	if got := ratelimit.Address(recorded); got != "203.0.113.4" {
		t.Errorf("recording the user changed the address to %q", got)
	}
	if got := ratelimit.Address(ratelimit.WithPrincipal(r, "01935000-0000-7000-8000-0000000000c3")); got != "" {
		t.Errorf("a user alone gave address %q", got)
	}
}

func TestAKeyedOnlyRouteHasNoAddressBucketAndIsRefusedByItsKey(t *testing.T) {
	reg := ratelimit.NewRegistry()
	route := reg.AddKeyed("POST   /teacher/students").
		WithKey("perActor", ratelimit.PrincipalKey, 10, ratelimit.PerMinute(2), ratelimit.PerHour(30))
	if found, ok := reg.Lookup("POST /teacher/students"); !ok || found != route {
		t.Fatalf("the keyed-only route is not registered under its normalized pattern: %v %v", found, ok)
	}
	if !slices.Contains(reg.Patterns(), "POST /teacher/students") {
		t.Errorf("Patterns %v does not list the keyed-only route", reg.Patterns())
	}
	if route.PerIP != nil {
		t.Error("a keyed-only route has a per-address bucket")
	}
	if len(route.Keyed) != 1 || route.Keyed[0].Name != "perActor" {
		t.Fatalf("keyed buckets %+v, want the one named perActor", route.Keyed)
	}
	bucket := route.Keyed[0]
	if got := bucket.Limiter.Rules(); len(got) != 2 || got[0] != (ratelimit.Rule{Burst: 2, Window: time.Minute}) || got[1] != (ratelimit.Rule{Burst: 30, Window: time.Hour}) {
		t.Errorf("perActor rules %+v", got)
	}

	r := httptest.NewRequest(http.MethodPost, "/teacher/students", nil)
	lan := ratelimit.WithPrincipal(ratelimit.WithAddress(r, "203.0.113.4"), "lan")
	minh := ratelimit.WithPrincipal(ratelimit.WithAddress(r, "203.0.113.4"), "minh")
	for i := 1; i <= 2; i++ {
		if allowed, _ := bucket.Limiter.Allow(bucket.Key(lan)); !allowed {
			t.Fatalf("request %d of one user was refused within the budget", i)
		}
	}
	if allowed, retry := bucket.Limiter.Allow(bucket.Key(lan)); allowed || retry <= 0 {
		t.Errorf("the third request of one user: allowed %v, retry %v, want it refused with a wait", allowed, retry)
	}
	if allowed, _ := bucket.Limiter.Allow(bucket.Key(minh)); !allowed {
		t.Error("another user behind the same address was refused by the first one's budget")
	}
}

func TestAddStillGivesEveryRouteItsAddressBucket(t *testing.T) {
	route := ratelimit.NewRegistry().Add("POST /join/preview", 10, ratelimit.PerMinute(120))
	if route.PerIP == nil {
		t.Fatal("Add registered a route with no per-address bucket")
	}
	if len(route.Keyed) != 0 {
		t.Errorf("Add registered keyed buckets %+v", route.Keyed)
	}
}
