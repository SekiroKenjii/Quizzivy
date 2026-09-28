package httpx_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/platform/ratelimit"
)

func limitedLogin(reg *ratelimit.Registry, bodies *[]string) http.Handler {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		*bodies = append(*bodies, string(body))
		w.WriteHeader(http.StatusNoContent)
	})
	return httpx.RateLimit(reg, ratelimit.ClientIP(""))(next)
}

func login(h http.Handler, ip, email string) int {
	r := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"`+email+`","password":"x"}`))
	r.Pattern = "POST /auth/login"
	r.RemoteAddr = ip + ":1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec.Code
}

func TestEveryKeyedBucketIsConsultedAndTheHandlerKeepsTheBody(t *testing.T) {
	reg := ratelimit.NewRegistry()
	email := ratelimit.JSONFieldKey("email", 4096)
	reg.Add("POST /auth/login", 100, ratelimit.PerMinute(100)).
		WithKey("perAddressAndEmail", ratelimit.Compose(ratelimit.Address, email), 100, ratelimit.PerMinute(2)).
		WithKey("perEmail", email, 100, ratelimit.PerMinute(3))
	var bodies []string
	h := limitedLogin(reg, &bodies)

	for i, ip := range []string{"198.51.100.1", "198.51.100.1"} {
		if got := login(h, ip, "an@example.com"); got != http.StatusNoContent {
			t.Fatalf("login %d: %d", i+1, got)
		}
	}
	if got := login(h, "198.51.100.1", "an@example.com"); got != http.StatusTooManyRequests {
		t.Errorf("the third login for one email from one address: %d, want the address-and-email bucket's 429", got)
	}
	if got := login(h, "198.51.100.2", "an@example.com"); got != http.StatusNoContent {
		t.Errorf("the same email from another address: %d, want it through", got)
	}
	if got := login(h, "198.51.100.3", "an@example.com"); got != http.StatusTooManyRequests {
		t.Errorf("the fourth login for one email across addresses: %d, want the per-email bucket's 429", got)
	}
	if got := login(h, "198.51.100.1", "binh@example.com"); got != http.StatusNoContent {
		t.Errorf("another email from the first address: %d, want it through", got)
	}
	for _, body := range bodies {
		if !strings.Contains(body, `"password":"x"`) {
			t.Errorf("the handler read %q: a body key consumed the body", body)
		}
	}
}

func TestAnEmptyKeySkipsItsBucket(t *testing.T) {
	reg := ratelimit.NewRegistry()
	reg.Add("POST /auth/login", 100, ratelimit.PerMinute(100)).
		WithKey("perEmail", ratelimit.JSONFieldKey("email", 4096), 100, ratelimit.PerMinute(1))
	var bodies []string
	h := limitedLogin(reg, &bodies)
	for i := range 3 {
		r := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"password":"x"}`))
		r.Pattern = "POST /auth/login"
		r.RemoteAddr = "198.51.100.9:1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("a body with no email, request %d: %d, want the per-email bucket skipped", i+1, rec.Code)
		}
	}
}
