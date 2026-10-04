package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"quizzivy/internal/platform/httpx"
)

type unroutedAnswer struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func unrouted(t *testing.T, acceptLanguage string, write func(http.ResponseWriter, *http.Request)) (*httptest.ResponseRecorder, unroutedAnswer) {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, "/auth/me", nil)
	if acceptLanguage != "" {
		req.Header.Set("Accept-Language", acceptLanguage)
	}
	rec := httptest.NewRecorder()
	write(rec, req)
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Accept-Language %q: Content-Type = %q, want application/json", acceptLanguage, got)
	}
	var answer unroutedAnswer
	if err := json.NewDecoder(rec.Body).Decode(&answer); err != nil {
		t.Fatalf("Accept-Language %q: response is not the error envelope: %v", acceptLanguage, err)
	}
	return rec, answer
}

func TestWriteNotFoundAnswers404InTheCallersLanguage(t *testing.T) {
	for acceptLanguage, want := range map[string]string{
		"":   "Không tìm thấy đường dẫn.",
		"en": "The path was not found.",
	} {
		rec, answer := unrouted(t, acceptLanguage, httpx.WriteNotFound)
		if rec.Code != http.StatusNotFound {
			t.Errorf("Accept-Language %q: status = %d, want 404", acceptLanguage, rec.Code)
		}
		if answer.Error.Code != string(httpx.CodeNotFound) || answer.Error.Message != want {
			t.Errorf("Accept-Language %q: answer = %s %q, want NOT_FOUND %q", acceptLanguage, answer.Error.Code, answer.Error.Message, want)
		}
		if got, sent := rec.Header()["Allow"]; sent {
			t.Errorf("Accept-Language %q: Allow = %q on a 404, want none", acceptLanguage, got)
		}
	}
}

func TestWriteMethodNotAllowedAnswers405AndNamesTheMethods(t *testing.T) {
	for acceptLanguage, want := range map[string]string{
		"":   "Phương thức này không dùng được cho đường dẫn này.",
		"en": "This method is not allowed for this path.",
	} {
		rec, answer := unrouted(t, acceptLanguage, func(w http.ResponseWriter, r *http.Request) {
			httpx.WriteMethodNotAllowed(w, r, []string{http.MethodGet, http.MethodPatch})
		})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("Accept-Language %q: status = %d, want 405", acceptLanguage, rec.Code)
		}
		if answer.Error.Code != "METHOD_NOT_ALLOWED" || answer.Error.Message != want {
			t.Errorf("Accept-Language %q: answer = %s %q, want METHOD_NOT_ALLOWED %q", acceptLanguage, answer.Error.Code, answer.Error.Message, want)
		}
		if got := rec.Header().Get("Allow"); got != "GET, PATCH" {
			t.Errorf("Accept-Language %q: Allow = %q, want %q", acceptLanguage, got, "GET, PATCH")
		}
	}
}
