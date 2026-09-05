package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The per-IP key is what makes §6.5 work. If a client can choose it, every
// bucket is empty and the limit protects nothing.

func req(remote string, headers map[string]string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/join/preview", nil)
	r.RemoteAddr = remote
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}

func TestFallsBackToTheSocketWhenNoHeaderIsConfigured(t *testing.T) {
	got := ClientIP("")(req("203.0.113.7:5555", map[string]string{
		"X-Forwarded-For":  "1.2.3.4",
		"CF-Connecting-IP": "5.6.7.8",
	}))
	if got != "203.0.113.7" {
		t.Errorf("got %q; with no header configured, only the socket may be trusted", got)
	}
}

func TestUsesTheNamedHeader(t *testing.T) {
	got := ClientIP("CF-Connecting-IP")(req("10.0.0.1:5555", map[string]string{
		"CF-Connecting-IP": "203.0.113.9",
	}))
	if got != "203.0.113.9" {
		t.Errorf("got %q, want the CF-Connecting-IP value", got)
	}
}

func TestAClientCannotForgeTheKeyByPresettingXForwardedFor(t *testing.T) {
	key := ClientIP("CF-Connecting-IP")

	forged := req("10.0.0.1:5555", map[string]string{
		"X-Forwarded-For":  "198.51.100.99, 203.0.113.9",
		"CF-Connecting-IP": "203.0.113.9",
	})
	honest := req("10.0.0.1:5555", map[string]string{
		"X-Forwarded-For":  "203.0.113.9",
		"CF-Connecting-IP": "203.0.113.9",
	})

	if key(forged) != key(honest) {
		t.Errorf("a client changed its own rate-limit key by setting X-Forwarded-For: %q vs %q",
			key(forged), key(honest))
	}
	if key(forged) == "198.51.100.99" {
		t.Error("the forged X-Forwarded-For value was used as the key")
	}
}

func TestSpoofingTheSameHeaderIsHarmlessBecauseTheProxyOverwritesIt(t *testing.T) {
	key := ClientIP("Fly-Client-IP")
	a := key(req("10.0.0.1:5555", map[string]string{"Fly-Client-IP": "203.0.113.9"}))
	b := key(req("10.0.0.1:5555", map[string]string{"Fly-Client-IP": "203.0.113.9"}))
	if a != b || a != "203.0.113.9" {
		t.Errorf("unstable key: %q, %q", a, b)
	}
}

func TestExhaustionIsNotEscapableByRotatingXForwardedFor(t *testing.T) {
	// End to end: burn the budget, then try to get more by forging.
	l := New(100, PerMinute(3))
	key := ClientIP("CF-Connecting-IP")

	for i := 0; i < 3; i++ {
		l.Allow(key(req("10.0.0.1:1", map[string]string{"CF-Connecting-IP": "203.0.113.9"})))
	}
	for _, forged := range []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"} {
		r := req("10.0.0.1:1", map[string]string{
			"X-Forwarded-For":  forged,
			"CF-Connecting-IP": "203.0.113.9",
		})
		if ok, _ := l.Allow(key(r)); ok {
			t.Fatalf("forging X-Forwarded-For: %s bought another request", forged)
		}
	}
}
