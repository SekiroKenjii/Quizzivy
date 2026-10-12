package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"quizzivy/internal/platform/httpx"
)

const caller = "01935000-0000-7000-8000-0000000000c3"

type serving func(ctx context.Context, w http.ResponseWriter) error

func answered(t *testing.T, acceptLanguage string, serve serving) *httptest.ResponseRecorder {
	t.Helper()
	handler := httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(
		httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
			return httpx.Principal{UserID: caller}, nil
		})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := serve(r.Context(), w); err != nil {
				t.Fatal(err)
			}
		})))
	request := httptest.NewRequest(http.MethodGet, "/me/notifications", nil)
	request.Header.Set("Authorization", "Bearer fixture")
	if acceptLanguage != "" {
		request.Header.Set("Accept-Language", acceptLanguage)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
