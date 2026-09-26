package apidocs_test

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"runtime"
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

var inlineScript = regexp.MustCompile(`(?s)<script>(.*?)</script>`)

func TestTheReferencePagePinsScalarByIntegrityAndAllowsOnlyItsOwnScripts(t *testing.T) {
	rec := httptest.NewRecorder()
	apidocs.Reference("/docs/openapi.json").ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/docs", nil))
	body := rec.Body.String()

	tag := `<script src="` + apidocs.ScalarURL + `" integrity="` + apidocs.ScalarIntegrity + `" crossorigin="anonymous"></script>`
	if !strings.Contains(body, tag) {
		t.Fatalf("the bundle is not loaded with integrity and crossorigin:\n%s", body)
	}
	inline := inlineScript.FindStringSubmatch(body)
	if inline == nil {
		t.Fatal("no inline script")
	}
	digest := sha256.Sum256([]byte(inline[1]))
	policy := rec.Header().Get("Content-Security-Policy")
	want := "script-src 'sha256-" + base64.StdEncoding.EncodeToString(digest[:]) + "' " + apidocs.ScalarURL + ";"
	if !strings.Contains(policy, want) || !strings.Contains(policy, "connect-src 'self'") || !strings.Contains(policy, "default-src 'none'") {
		t.Fatalf("policy %q does not admit exactly the pinned bundle and the inline script", policy)
	}
	if !strings.Contains(inline[1], "servers: [{ url: window.location.origin }]") {
		t.Fatal("Try it would default to the contract's first server, which the page's CSP refuses")
	}
	for _, option := range []string{"withDefaultFonts: false", "telemetry: false", "agent: { disabled: true }", "mcp: { disabled: true }"} {
		if !strings.Contains(inline[1], option) {
			t.Fatalf("Scalar is left to call third parties: %q is missing", option)
		}
	}
}

func TestTheIntegrityIsDeclaredBesideTheVersionItPins(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	source := filepath.Join(filepath.Dir(file), "..", "apidocs.go")
	parsed, err := parser.ParseFile(token.NewFileSet(), source, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range parsed.Decls {
		group, ok := decl.(*ast.GenDecl)
		if !ok || group.Tok != token.CONST {
			continue
		}
		names := map[string]bool{}
		for _, spec := range group.Specs {
			for _, name := range spec.(*ast.ValueSpec).Names {
				names[name.Name] = true
			}
		}
		if names["ScalarVersion"] && names["ScalarIntegrity"] {
			return
		}
	}
	t.Fatal("ScalarVersion and ScalarIntegrity must share one const block so a bump updates both")
}
