package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"quizzivy/gen/openapi"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
)

func TestBlankReasonSpeaksTheCallersLanguage(t *testing.T) {
	for _, c := range []struct {
		acceptLanguage string
		message        string
		reason         string
	}{
		{"", "Cần ghi lý do.", "Lý do không được để trống."},
		{"en", "A reason is needed.", "The reason cannot be empty."},
	} {
		var answer openapi.ErrorResponse
		handler := httpx.WithRequestMeta(func(*http.Request) string { return "" })(
			http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				answer = httpapi.BlankReason(r.Context())
			}))
		request := httptest.NewRequest(http.MethodPost, "/", nil)
		if c.acceptLanguage != "" {
			request.Header.Set("Accept-Language", c.acceptLanguage)
		}
		handler.ServeHTTP(httptest.NewRecorder(), request)

		if answer.Error.Code != openapi.VALIDATIONFAILED {
			t.Errorf("Accept-Language %q: code = %q, want VALIDATION_FAILED", c.acceptLanguage, answer.Error.Code)
		}
		if answer.Error.Message != c.message {
			t.Errorf("Accept-Language %q: message = %q, want %q", c.acceptLanguage, answer.Error.Message, c.message)
		}
		if answer.Error.Details == nil {
			t.Fatalf("Accept-Language %q: the answer carries no details", c.acceptLanguage)
		}
		if got := (*answer.Error.Details)["reason"]; got != c.reason {
			t.Errorf("Accept-Language %q: details.reason = %q, want %q", c.acceptLanguage, got, c.reason)
		}
	}
}
