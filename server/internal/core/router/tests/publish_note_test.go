package router_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublishTestTakesAnOptionalNoteOfAtMostTwoHundredCharacters(t *testing.T) {
	issuer := testIssuer(t)
	handler := newAuthTestRouter(t, issuer)
	token, err := issuer.Issue("01935000-0000-7000-8000-000000000001", "admin", 0)
	if err != nil {
		t.Fatal(err)
	}
	publish := func(body string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodPost, "/teacher/tests/01935000-0000-7000-8000-000000000002/publish", strings.NewReader(body))
		if body != "" {
			request.Header.Set("Content-Type", "application/json")
		}
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}

	for name, body := range map[string]string{
		"no body":              "",
		"an empty object":      `{}`,
		"a null note":          `{"changeNote":null}`,
		"a short note":         `{"changeNote":"Sửa câu 2"}`,
		"two hundred letters":  `{"changeNote":"` + strings.Repeat("đ", 200) + `"}`,
		"an empty note string": `{"changeNote":""}`,
	} {
		t.Run(name, func(t *testing.T) {
			if response := publish(body); response.Code == http.StatusBadRequest {
				t.Fatalf("a valid body was refused: %s", response.Body.String())
			}
		})
	}
	for name, body := range map[string]string{
		"two hundred and one letters": `{"changeNote":"` + strings.Repeat("đ", 201) + `"}`,
		"a number":                    `{"changeNote":12}`,
		"an unknown field":            `{"changeNote":"x","publishedBy":"someone"}`,
		"not json":                    `pretzel`,
	} {
		t.Run(name, func(t *testing.T) {
			response := publish(body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", response.Code, response.Body.String())
			}
			if got := errorCode(t, response); got != "VALIDATION_FAILED" {
				t.Errorf("error code = %q, want VALIDATION_FAILED", got)
			}
		})
	}
}
