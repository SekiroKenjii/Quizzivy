package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"quizzivy/internal/platform/httpx"
)

func TestRequestLimitPreservesOrdinaryBodiesAndStreamingUploads(t *testing.T) {
	for _, test := range []struct {
		name    string
		pattern string
		body    string
	}{
		{name: "small JSON", pattern: "POST /auth/login", body: `{"email":"student@example.com"}`},
		{name: "streaming media", pattern: "POST /admin/media", body: strings.Repeat("upload", 1000)},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(test.body))
			request.Pattern = test.pattern
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body strings.Builder
				buffer := make([]byte, 8192)
				n, err := r.Body.Read(buffer)
				if err != nil {
					t.Fatal(err)
				}
				body.Write(buffer[:n])
				if body.String() != test.body {
					t.Error("middleware changed request body")
				}
				w.WriteHeader(http.StatusNoContent)
			})
			recorder := httptest.NewRecorder()
			httpx.LimitRequestBody(map[string]struct{}{"POST /admin/media": {}}, 100, nil)(next).ServeHTTP(recorder, request)
			if recorder.Code != http.StatusNoContent {
				t.Fatalf("status=%d", recorder.Code)
			}
		})
	}
}
