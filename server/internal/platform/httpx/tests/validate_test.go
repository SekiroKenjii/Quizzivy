package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"quizzivy/internal/platform/httpx"
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

func TestAMalformedParameterWithoutANameGetsTheGenericSentence(t *testing.T) {
	for acceptLanguage, want := range map[string]string{
		"":   "Dữ liệu gửi lên không hợp lệ.",
		"en": "The submitted data is not valid.",
	} {
		for writer, write := range map[string]func(http.ResponseWriter, *http.Request){
			"WriteMalformedBody": httpx.WriteMalformedBody,
			"WriteMalformedParameter": func(w http.ResponseWriter, r *http.Request) {
				httpx.WriteMalformedParameter(w, r, "")
			},
		} {
			req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
			if acceptLanguage != "" {
				req.Header.Set("Accept-Language", acceptLanguage)
			}
			rec := httptest.NewRecorder()
			write(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("%s, Accept-Language %q: status = %d, want 400", writer, acceptLanguage, rec.Code)
			}
			var body struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("%s, Accept-Language %q: response is not the error envelope: %v", writer, acceptLanguage, err)
			}
			if body.Error.Code != "VALIDATION_FAILED" || body.Error.Message != want {
				t.Errorf("%s, Accept-Language %q: answer = %s %q, want VALIDATION_FAILED %q", writer, acceptLanguage, body.Error.Code, body.Error.Message, want)
			}
		}
	}
}
