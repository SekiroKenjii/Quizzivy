package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"quizzivy/internal/platform/httpx"
)

func TestTextFollowsAcceptLanguageAndDefaultsToVietnamese(t *testing.T) {
	for _, c := range []struct {
		accept string
		want   string
	}{
		{"", "vi"},
		{"vi", "vi"},
		{"en", "en"},
		{"en-US,en;q=0.9,vi;q=0.8", "en"},
		{"vi,en;q=0.9", "vi"},
		{"fr", "vi"},
		{"fr,en;q=0.5", "en"},
	} {
		got := "the handler did not run"
		handler := httpx.WithRequestMeta(func(*http.Request) string { return "" })(
			http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = httpx.Text(r.Context(), "vi", "en")
			}))
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		if c.accept != "" {
			request.Header.Set("Accept-Language", c.accept)
		}
		handler.ServeHTTP(httptest.NewRecorder(), request)
		if got != c.want {
			t.Errorf("Text with Accept-Language %q answered %q, want %q", c.accept, got, c.want)
		}
	}

	bare := httptest.NewRequest(http.MethodGet, "/docs", nil)
	if got := httpx.TextFor(bare, "vi", "en"); got != "vi" {
		t.Errorf("TextFor without Accept-Language answered %q, want vi", got)
	}
	bare.Header.Set("Accept-Language", "en")
	if got := httpx.TextFor(bare, "vi", "en"); got != "en" {
		t.Errorf("TextFor with Accept-Language en answered %q, want en", got)
	}
}
