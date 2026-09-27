package router_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"quizzivy/internal/core/router"
	identitytoken "quizzivy/internal/modules/identity/application/token"
)

var theV070AdminOperations = []string{
	"DELETE /admin/assignments/{id}",
	"DELETE /admin/classes/{id}",
	"DELETE /admin/classes/{id}/join-code",
	"DELETE /admin/classes/{id}/members/{userId}",
	"DELETE /admin/media/{id}",
	"DELETE /admin/question-groups/{id}",
	"DELETE /admin/questions/{id}",
	"DELETE /admin/students/{id}",
	"DELETE /admin/tests/{id}",
	"DELETE /admin/tests/{id}/versions/{version}",
	"GET /admin/assignments",
	"GET /admin/assignments/{id}",
	"GET /admin/assignments/{id}/answers",
	"GET /admin/assignments/{id}/attempts",
	"GET /admin/attempts",
	"GET /admin/attempts/{id}",
	"GET /admin/attempts/{id}/events",
	"GET /admin/classes",
	"GET /admin/classes/{id}",
	"GET /admin/classes/{id}/members",
	"GET /admin/dashboard",
	"GET /admin/imports",
	"GET /admin/imports/capabilities",
	"GET /admin/imports/limits",
	"GET /admin/imports/{id}",
	"GET /admin/imports/{id}/review",
	"GET /admin/imports/{id}/source",
	"GET /admin/imports/{id}/sources/{sourceId}/download",
	"GET /admin/media",
	"GET /admin/question-groups",
	"GET /admin/question-groups/{id}",
	"GET /admin/questions",
	"GET /admin/questions/{id}",
	"GET /admin/students",
	"GET /admin/students/{id}",
	"GET /admin/tests",
	"GET /admin/tests/{id}",
	"GET /admin/tests/{id}/preview",
	"GET /admin/tests/{id}/versions",
	"PATCH /admin/assignments/{id}",
	"PATCH /admin/attempts/{id}/note",
	"PATCH /admin/classes/{id}",
	"PATCH /admin/question-groups/{id}/archive",
	"PATCH /admin/questions/{id}",
	"PATCH /admin/students/{id}",
	"PATCH /admin/tests/{id}",
	"POST /admin/assignments",
	"POST /admin/assignments/{id}/reopen",
	"POST /admin/attempts/{id}/extend",
	"POST /admin/attempts/{id}/finish-grading",
	"POST /admin/attempts/{id}/flag",
	"POST /admin/attempts/{id}/grade",
	"POST /admin/attempts/{id}/reset",
	"POST /admin/attempts/{id}/void",
	"POST /admin/classes",
	"POST /admin/classes/{id}/join-code",
	"POST /admin/classes/{id}/members",
	"POST /admin/imports",
	"POST /admin/imports/{id}/cancel",
	"POST /admin/imports/{id}/commit",
	"POST /admin/imports/{id}/process",
	"POST /admin/imports/{id}/review/adopt",
	"POST /admin/imports/{id}/sources",
	"POST /admin/media",
	"POST /admin/question-groups",
	"POST /admin/question-groups/{id}/copy",
	"POST /admin/questions",
	"POST /admin/questions/tags",
	"POST /admin/questions/{id}/duplicate",
	"POST /admin/students",
	"POST /admin/students/{id}/reset-password",
	"POST /admin/tests",
	"POST /admin/tests/{id}/duplicate",
	"POST /admin/tests/{id}/publish",
	"POST /admin/tests/{id}/versions/{version}/current",
	"POST /admin/tests/{id}/versions/{version}/draft",
	"PUT /admin/imports/{id}/review",
	"PUT /admin/question-groups/{id}",
}

var wildcard = regexp.MustCompile(`\{[^/]+\}`)

func sample(schema *openapi3.Schema) string {
	switch {
	case len(schema.Enum) > 0:
		return fmt.Sprint(schema.Enum[0])
	case schema.Type.Is(openapi3.TypeInteger):
		return "1"
	case schema.Format == "date-time":
		return "2026-09-28T00:00:00Z"
	}
	return "01935000-0000-7000-8000-0000000000e1"
}

func filler(t *testing.T, spec *openapi3.T, method, target string) func(string) string {
	t.Helper()
	item := spec.Paths.Find(target)
	if item == nil || item.GetOperation(method) == nil {
		t.Fatalf("%s %s is not in the contract", method, target)
	}
	values := map[string]string{}
	query := url.Values{}
	for _, param := range append(slices.Clone(item.Parameters), item.GetOperation(method).Parameters...) {
		value := sample(param.Value.Schema.Value)
		switch {
		case param.Value.In == openapi3.ParameterInPath:
			values["{"+param.Value.Name+"}"] = value
		case param.Value.In == openapi3.ParameterInQuery && param.Value.Required:
			query.Set(param.Value.Name, value)
		}
	}
	return func(path string) string {
		out := wildcard.ReplaceAllStringFunc(path, func(name string) string { return values[name] })
		if len(query) > 0 {
			out += "?" + query.Encode()
		}
		return out
	}
}

func splitPattern(pattern string) (string, string) {
	method, path, _ := strings.Cut(pattern, " ")
	return method, path
}

type logLines struct{ buf bytes.Buffer }

func (l *logLines) legacy(t *testing.T) []map[string]any {
	t.Helper()
	var out []map[string]any
	scanner := bufio.NewScanner(bytes.NewReader(l.buf.Bytes()))
	for scanner.Scan() {
		var line map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			t.Fatalf("log line %q: %v", scanner.Text(), err)
		}
		if line["msg"] == "legacy_admin_path" {
			out = append(out, line)
		}
	}
	return out
}

func loggedRouter(t *testing.T, issuer *identitytoken.Issuer, logs *logLines) http.Handler {
	t.Helper()
	h, err := router.New(router.Deps{Principals: rolePrincipals(), DB: fakeDB{}, Tokens: issuer},
		slog.New(slog.NewJSONHandler(&logs.buf, nil)), []string{"https://app.quizzivy.com"}, "")
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestTheLegacyTableIsTheV070AdminOperations(t *testing.T) {
	table := router.LegacyAdminPaths()
	keys := make([]string, 0, len(table))
	for pattern := range table {
		keys = append(keys, pattern)
	}
	sort.Strings(keys)
	want := slices.Clone(theV070AdminOperations)
	sort.Strings(want)
	if !slices.Equal(keys, want) {
		t.Fatalf("the alias table differs from v0.7.0's /admin operations:\n%s", lineDiff(strings.Join(want, "\n"), strings.Join(keys, "\n")))
	}
	current := map[string]bool{}
	eachOperation(freshSpec(t), func(pattern string, _ *openapi3.Operation) { current[pattern] = true })
	for old, target := range table {
		method, path := splitPattern(old)
		expected := "/teacher/" + strings.TrimPrefix(path, "/admin/")
		if old == "DELETE /admin/students/{id}" {
			expected = "/admin/users/{id}"
		}
		if target != expected {
			t.Errorf("%s → %s, want %s", old, target, expected)
		}
		if !current[method+" "+target] {
			t.Errorf("%s → %s, which the contract does not serve", old, method+" "+target)
		}
		if current[old] {
			t.Errorf("%s is still in the contract, so the alias would shadow it", old)
		}
	}
}

func TestEveryLegacyPathAnswersAsItsNewPathDoes(t *testing.T) {
	issuer := testIssuer(t)
	spec := freshSpec(t)
	for old, target := range router.LegacyAdminPaths() {
		method, path := splitPattern(old)
		fill := filler(t, spec, method, target)
		for _, user := range []string{studentUser, adminUser} {
			h := roleRouter(t, issuer, rolePrincipals())
			before := sendAs(t, h, issuer, method, fill(path), user, "")
			after := sendAs(t, h, issuer, method, fill(target), user, "")
			if before.Code != after.Code {
				t.Errorf("%s as %s: %d on the old path, %d on %s", old, user, before.Code, after.Code, target)
			}
			switch {
			case user == studentUser && before.Code != http.StatusForbidden:
				t.Errorf("%s as a student: %d, want 403", old, before.Code)
			case user == adminUser && (before.Code == http.StatusNotFound || before.Code == http.StatusMethodNotAllowed ||
				before.Code == http.StatusForbidden || before.Code == http.StatusUnauthorized):
				t.Errorf("%s as an Admin: %d, want it to reach the handler", old, before.Code)
			}
		}
	}
}

func TestTheSixthResetThroughAnOldPathIsLimitedUnderTheNewKey(t *testing.T) {
	issuer := testIssuer(t)
	h := roleRouter(t, issuer, rolePrincipals())
	id := "01935000-0000-7000-8000-0000000000e1"
	for i := range 5 {
		path := "/teacher/students/" + id + "/reset-password"
		if i%2 == 1 {
			path = "/admin/students/" + id + "/reset-password"
		}
		if rec := sendAs(t, h, issuer, http.MethodPost, path, adminUser, ""); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d through %s was limited early", i+1, path)
		}
	}
	rec := sendAs(t, h, issuer, http.MethodPost, "/admin/students/"+id+"/reset-password", adminUser, "")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("the sixth reset in a minute: %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("the 429 carries no Retry-After")
	}
}

func TestAPreflightToAnOldPathIsAnsweredAsTheNewPath(t *testing.T) {
	h := roleRouter(t, testIssuer(t), rolePrincipals())
	preflight := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodOptions, path, nil)
		req.Header.Set("Origin", "https://app.quizzivy.com")
		req.Header.Set("Access-Control-Request-Method", http.MethodPatch)
		req.Header.Set("Access-Control-Request-Headers", "authorization, content-type")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	old := preflight("/admin/tests/01935000-0000-7000-8000-0000000000e1")
	current := preflight("/teacher/tests/01935000-0000-7000-8000-0000000000e1")
	if old.Code != current.Code {
		t.Fatalf("preflight status %d on the old path, %d on the new", old.Code, current.Code)
	}
	for _, header := range []string{"Access-Control-Allow-Origin", "Access-Control-Allow-Methods", "Access-Control-Allow-Headers", "Access-Control-Allow-Credentials", "Access-Control-Max-Age", "Vary"} {
		if old.Header().Get(header) != current.Header().Get(header) {
			t.Errorf("%s: %q on the old path, %q on the new", header, old.Header().Get(header), current.Header().Get(header))
		}
	}
	if current.Header().Get("Access-Control-Allow-Origin") != "https://app.quizzivy.com" {
		t.Errorf("the preflight was not answered: %d %v", current.Code, current.Header())
	}
}

func TestOneLogLinePerRewrittenRequest(t *testing.T) {
	issuer := testIssuer(t)
	logs := &logLines{}
	h := loggedRouter(t, issuer, logs)
	sendAs(t, h, issuer, http.MethodGet, "/admin/tests/01935000-0000-7000-8000-0000000000e1", adminUser, "")
	lines := logs.legacy(t)
	if len(lines) != 1 {
		t.Fatalf("%d legacy_admin_path lines for one request, want 1", len(lines))
	}
	line := lines[0]
	if line["level"] != "INFO" || line["method"] != http.MethodGet || line["pattern"] != "GET /admin/tests/{id}" {
		t.Errorf("the line is %v", line)
	}
	if id, _ := line["request_id"].(string); id == "" {
		t.Errorf("the line carries no request id: %v", line)
	}
}

func TestPathsOutsideTheTablePassUntouched(t *testing.T) {
	issuer := testIssuer(t)
	logs := &logLines{}
	h := loggedRouter(t, issuer, logs)
	id := "01935000-0000-7000-8000-0000000000e1"
	for _, c := range []struct {
		method, path string
		want         int
	}{
		{http.MethodPost, "/admin/docs-session", 0},
		{http.MethodDelete, "/admin/users/" + id, 0},
		{http.MethodGet, "/teacher/tests/" + id, 0},
		{http.MethodGet, "/admin/nothing-here", http.StatusNotFound},
		{http.MethodPut, "/admin/tests/" + id, http.StatusNotFound},
		{http.MethodGet, "/admin/tests/" + id + "/extra/segments", http.StatusNotFound},
	} {
		rec := sendAs(t, h, issuer, c.method, c.path, adminUser, "")
		if c.want != 0 && rec.Code != c.want {
			t.Errorf("%s %s: %d, want %d", c.method, c.path, rec.Code, c.want)
		}
		if c.want == 0 && (rec.Code == http.StatusNotFound || rec.Code == http.StatusMethodNotAllowed) {
			t.Errorf("%s %s: %d, want it served", c.method, c.path, rec.Code)
		}
	}
	if lines := logs.legacy(t); len(lines) != 0 {
		t.Errorf("requests outside the table were rewritten: %v", lines)
	}
}

func TestAnEncodedSlashStaysInsideItsSegment(t *testing.T) {
	issuer := testIssuer(t)
	h := roleRouter(t, issuer, rolePrincipals())
	id := "01935000-0000-7000-8000-0000000000e1"
	old := sendAs(t, h, issuer, http.MethodDelete, "/admin/tests/"+id+"%2Fversions%2F3", adminUser, "")
	current := sendAs(t, h, issuer, http.MethodDelete, "/teacher/tests/"+id+"%2Fversions%2F3", adminUser, "")
	if old.Code != http.StatusBadRequest || current.Code != http.StatusBadRequest {
		t.Fatalf("an id holding an encoded slash: %d on the old path, %d on the new, want 400 on both", old.Code, current.Code)
	}
	if errorCode(t, old) != errorCode(t, current) {
		t.Errorf("error code %q on the old path, %q on the new", errorCode(t, old), errorCode(t, current))
	}
}
