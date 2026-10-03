package ratelimit_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"quizzivy/internal/platform/ratelimit"
)

func fixed(value string) ratelimit.KeyFunc {
	return func(*http.Request) string { return value }
}

func TestComposeJoinsItsPartsAndSkipsWhenOneIsMissing(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	a := ratelimit.Compose(fixed("198.51.100.7"), fixed("an@example.com"))(r)
	b := ratelimit.Compose(fixed("198.51.100.7"), fixed("binh@example.com"))(r)
	c := ratelimit.Compose(fixed("198.51.100.8"), fixed("an@example.com"))(r)
	if a == "" || a == b || a == c || b == c {
		t.Errorf("composite keys %q, %q, %q do not tell their parts apart", a, b, c)
	}
	if got := ratelimit.Compose(fixed("198.51.100.7"), fixed(""))(r); got != "" {
		t.Errorf("a missing part gave %q, want the bucket skipped", got)
	}
	if got := ratelimit.Compose(fixed(""), fixed("an@example.com"))(r); got != "" {
		t.Errorf("a missing first part gave %q, want the bucket skipped", got)
	}
	if ratelimit.Compose(fixed("ab"), fixed("c"))(r) == ratelimit.Compose(fixed("a"), fixed("bc"))(r) {
		t.Error("parts that concatenate alike share a bucket")
	}
}

func TestAddressIsWhatTheLimiterRecorded(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	if got := ratelimit.Address(r); got != "" {
		t.Errorf("an unrecorded request has address %q", got)
	}
	if got := ratelimit.Address(ratelimit.WithAddress(r, "203.0.113.4")); got != "203.0.113.4" {
		t.Errorf("recorded address read back as %q", got)
	}
}

func TestKeyedBucketsKeepTheirOrderAndRules(t *testing.T) {
	route := ratelimit.NewRegistry().Add("POST /auth/login", 10, ratelimit.PerMinute(120), ratelimit.PerHour(600)).
		WithKey("perAddressAndEmail", fixed("x"), 10, ratelimit.PerMinute(10)).
		WithKey("perEmail", fixed("y"), 10, ratelimit.PerHour(20))
	if len(route.Keyed) != 2 || route.Keyed[0].Name != "perAddressAndEmail" || route.Keyed[1].Name != "perEmail" {
		t.Fatalf("keyed buckets %+v", route.Keyed)
	}
	rules := route.Keyed[1].Limiter.Rules()
	if len(rules) != 1 || rules[0] != (ratelimit.Rule{Burst: 20, Window: time.Hour}) {
		t.Errorf("perEmail rules %+v", rules)
	}
	rules[0].Burst = 1
	if route.Keyed[1].Limiter.Rules()[0].Burst != 20 {
		t.Error("Rules hands out the limiter's own slice")
	}
	if got := route.PerIP.Rules(); len(got) != 2 || got[0] != (ratelimit.Rule{Burst: 120, Window: time.Minute}) || got[1] != (ratelimit.Rule{Burst: 600, Window: time.Hour}) {
		t.Errorf("per-address rules %+v", got)
	}
}
