package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const allowed = "https://app.quizzivy.com"

func corsHandler() http.Handler {
	return CORS([]string{allowed})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
}

func TestAllowedOriginGetsCredentialedHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Origin", allowed)
	rec := httptest.NewRecorder()
	corsHandler().ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != allowed {
		t.Errorf("Allow-Origin = %q, want %q", got, allowed)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Allow-Credentials = %q, want true", got)
	}
	// Without Vary, a shared cache can hand one origin's allow header to another.
	if rec.Header().Get("Vary") == "" {
		t.Error("Vary: Origin is missing")
	}
}

func TestUnlistedOriginGetsNoAllowHeader(t *testing.T) {
	for _, origin := range []string{
		"https://evil.example",
		// A near-miss: same suffix, different registrable domain.
		"https://app.quizzivy.com.evil.example",
		// Scheme matters.
		"http://app.quizzivy.com",
	} {
		req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()
		corsHandler().ServeHTTP(rec, req)

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("origin %q was allowed (%q); the allowlist must be exact", origin, got)
		}
		if rec.Header().Get("Vary") == "" {
			t.Errorf("origin %q: Vary must be set even when the origin is rejected", origin)
		}
	}
}

func TestPreflightIsAnswered(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/app/attempts/x/answers", nil)
	req.Header.Set("Origin", allowed)
	req.Header.Set("Access-Control-Request-Method", "PATCH")
	req.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
	rec := httptest.NewRecorder()
	corsHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d, want 204", rec.Code)
	}
	for _, want := range []string{"PATCH", "DELETE"} {
		if m := rec.Header().Get("Access-Control-Allow-Methods"); !contains(m, want) {
			t.Errorf("Allow-Methods %q is missing %s", m, want)
		}
	}
	if h := rec.Header().Get("Access-Control-Allow-Headers"); !contains(h, "Authorization") {
		t.Errorf("Allow-Headers %q is missing Authorization", h)
	}
}

func TestWildcardIsNeverEmitted(t *testing.T) {
	h := CORS([]string{"*"})(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Origin", "https://anything.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q; '*' must never be emitted", got)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle ||
		len(haystack) > 0 && indexOf(haystack, needle) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
