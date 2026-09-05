package apidocs_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"quizzivy/internal/platform/apidocs"
)

func TestTheReferencePageLoadsScalarAgainstTheServedSpec(t *testing.T) {
	rec := httptest.NewRecorder()
	apidocs.Reference("/docs/openapi.json").ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/docs", nil))

	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("status %d content-type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	body := rec.Body.String()
	if !strings.Contains(body, "@scalar/api-reference@") || !strings.Contains(body, `"/docs/openapi.json"`) {
		t.Fatalf("page does not load Scalar against the served spec:\n%s", body)
	}
}

func TestTheSpecEndpointServesTheDocumentAsJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	apidocs.Spec(func() ([]byte, error) { return []byte(`{"openapi":"3.1.0"}`), nil }).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/docs/openapi.json", nil))

	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("status %d content-type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil || doc["openapi"] != "3.1.0" {
		t.Fatalf("body %q is not the document", rec.Body.String())
	}

	broken := httptest.NewRecorder()
	apidocs.Spec(func() ([]byte, error) { return nil, errors.New("gone") }).
		ServeHTTP(broken, httptest.NewRequest(http.MethodGet, "/docs/openapi.json", nil))
	if broken.Code != http.StatusInternalServerError {
		t.Fatalf("an unreadable document answered %d, want 500", broken.Code)
	}
}
