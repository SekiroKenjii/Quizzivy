package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPathOutsideTheContractIsRefusedInTheCallersLanguage(t *testing.T) {
	handler := validatorUnderTest(t)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("a path outside the contract reached the handler")
	}))

	for acceptLanguage, want := range map[string]string{
		"":   "Không tìm thấy đường dẫn.",
		"en": "The path was not found.",
	} {
		req := httptest.NewRequest(http.MethodGet, "/no-such-path", nil)
		if acceptLanguage != "" {
			req.Header.Set("Accept-Language", acceptLanguage)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("Accept-Language %q: status = %d, want 404", acceptLanguage, rec.Code)
		}
		var body struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("Accept-Language %q: response is not the error envelope: %v", acceptLanguage, err)
		}
		if body.Error.Code != "NOT_FOUND" || body.Error.Message != want {
			t.Errorf("Accept-Language %q: answer = %s %q, want NOT_FOUND %q", acceptLanguage, body.Error.Code, body.Error.Message, want)
		}
	}
}
