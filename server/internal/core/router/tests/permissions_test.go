package router_test

import (
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"quizzivy/gen/openapi"
	"quizzivy/internal/platform/httpx"
)

var updateGolden = flag.Bool("update", false, "rewrite testdata/permissions.golden from api/openapi.yaml")

func freshSpec(t *testing.T) *openapi3.T {
	t.Helper()
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	return spec
}

func eachOperation(spec *openapi3.T, fn func(pattern string, op *openapi3.Operation)) {
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			if op != nil {
				fn(method+" "+path, op)
			}
		}
	}
}

func openPatterns(spec *openapi3.T) map[string]struct{} {
	return httpx.OpenRoutes(spec, "bearerAuth")
}

func TestEveryNonOpenOperationDeclaresAPermission(t *testing.T) {
	spec := freshSpec(t)
	open := openPatterns(spec)
	eachOperation(spec, func(pattern string, op *openapi3.Operation) {
		if _, isOpen := open[pattern]; isOpen {
			return
		}
		if _, ok := op.Extensions[httpx.PermissionExtension]; !ok {
			t.Errorf("%s (%s) declares no x-permission", pattern, op.OperationID)
		}
	})
}

func TestOpenOperationsDeclareNone(t *testing.T) {
	spec := freshSpec(t)
	open := openPatterns(spec)
	if len(open) != 7 {
		t.Errorf("%d open operations, want the seven of theOpenSeven", len(open))
	}
	eachOperation(spec, func(pattern string, op *openapi3.Operation) {
		if _, isOpen := open[pattern]; !isOpen {
			return
		}
		if v, ok := op.Extensions[httpx.PermissionExtension]; ok {
			t.Errorf("open operation %s declares x-permission %v", pattern, v)
		}
	})
}

func TestEveryDeclaredValueIsKnown(t *testing.T) {
	requirements, err := httpx.PermissionRequirements(freshSpec(t), "bearerAuth")
	if err != nil {
		t.Fatal(err)
	}
	if len(requirements) != 96 {
		t.Errorf("%d operations declare a permission, want 96", len(requirements))
	}
	for pattern, requirement := range requirements {
		for _, k := range requirement.Keys() {
			if !k.Known() && !k.Pseudo() {
				t.Errorf("%s requires %q, which is neither a catalogue key nor a pseudo-key", pattern, k)
			}
		}
	}
}

func TestTheAssertionRefusesAnUndeclaredOperation(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		method  string
		value   any
		missing bool
		want    string
	}{
		{name: "undeclared", path: "/admin/tests", method: "GET", missing: true, want: "GET /admin/tests: declares no permission"},
		{name: "open and declared", path: "/auth/login", method: "POST", value: "self", want: "POST /auth/login: an open operation declares a permission"},
		{name: "unknown key", path: "/admin/media", method: "GET", value: "content.everything", want: `GET /admin/media: "content.everything" is neither a catalogue key nor a pseudo-key`},
		{name: "empty list", path: "/admin/classes", method: "GET", value: []any{}, want: "GET /admin/classes: declares an empty list"},
		{name: "repeated key", path: "/admin/students", method: "GET", value: []any{"people.students.read", "people.students.read"}, want: `GET /admin/students: repeats "people.students.read"`},
		{name: "hidden key outside admin", path: "/app/classes", method: "GET", value: "scope.all", want: `GET /app/classes: the hidden key "scope.all" is declared outside /admin/`},
		{name: "not a key", path: "/admin/dashboard", method: "GET", value: 7, want: "GET /admin/dashboard: declares 7, which is neither a key nor a list of keys"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			spec := freshSpec(t)
			op := spec.Paths.Find(c.path).GetOperation(c.method)
			if op == nil {
				t.Fatalf("no %s %s in the contract", c.method, c.path)
			}
			if c.missing {
				delete(op.Extensions, httpx.PermissionExtension)
			} else {
				if op.Extensions == nil {
					op.Extensions = map[string]any{}
				}
				op.Extensions[httpx.PermissionExtension] = c.value
			}
			_, err := httpx.PermissionRequirements(spec, "bearerAuth")
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want it to name %q", err, c.want)
			}
		})
	}
}

func TestThePermissionMapIsPinned(t *testing.T) {
	requirements, err := httpx.PermissionRequirements(freshSpec(t), "bearerAuth")
	if err != nil {
		t.Fatal(err)
	}
	lines := make([]string, 0, len(requirements))
	for pattern, requirement := range requirements {
		keys := make([]string, 0, len(requirement.Keys()))
		for _, k := range requirement.Keys() {
			keys = append(keys, string(k))
		}
		lines = append(lines, pattern+" "+strings.Join(keys, " | "))
	}
	sort.Strings(lines)
	got := strings.Join(lines, "\n") + "\n"
	golden := filepath.Join("testdata", "permissions.golden")
	if *updateGolden {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v; run go test -run TestThePermissionMapIsPinned -update", err)
	}
	if got != string(want) {
		t.Errorf("the permission map differs from %s; if the change is intended, run go test -run TestThePermissionMapIsPinned -update and review the diff\n%s",
			golden, lineDiff(string(want), got))
	}
}

func lineDiff(want, got string) string {
	wantSet := map[string]bool{}
	for _, l := range strings.Split(want, "\n") {
		wantSet[l] = true
	}
	gotSet := map[string]bool{}
	for _, l := range strings.Split(got, "\n") {
		gotSet[l] = true
	}
	var out []string
	for l := range wantSet {
		if !gotSet[l] && l != "" {
			out = append(out, "- "+l)
		}
	}
	for l := range gotSet {
		if !wantSet[l] && l != "" {
			out = append(out, "+ "+l)
		}
	}
	sort.Strings(out)
	return strings.Join(out, "\n")
}
