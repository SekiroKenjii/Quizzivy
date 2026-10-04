package core_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The dependency rules of the modular monolith, checked on every package under
// internal/. A test directory may import anything: tests exercise the public
// surface of whatever they need.
func TestPackagesOnlyDependInTheAllowedDirection(t *testing.T) {
	root := filepath.Join("..", "..")
	var offences []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
		if inTestDirectory(rel) {
			return nil
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range file.Imports {
			target := strings.Trim(imp.Path.Value, `"`)
			if !strings.HasPrefix(target, "quizzivy/internal/") {
				continue
			}
			if reason := forbidden(rel, strings.TrimPrefix(target, "quizzivy/internal/")); reason != "" {
				offences = append(offences, rel+" imports "+target+": "+reason)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(offences)
	for _, o := range offences {
		t.Error(o)
	}
}

func TestOnlyTestDirectoriesAreSkipped(t *testing.T) {
	cases := []struct {
		rel  string
		skip bool
	}{
		{"modules/tests/domain/publish.go", false},
		{"modules/tests/repositories/publish_snapshot.go", false},
		{"modules/tests/http/version_management.go", false},
		{"modules/tests/application/command/publish.go", false},
		{"modules/tests/application/tests/support.go", true},
		{"modules/tests/tests/support.go", true},
		{"modules/media/repositories/postgres.go", false},
		{"modules/media/repositories/tests/support.go", true},
		{"platform/db/tests/support.go", true},
		{"core/tests/support.go", true},
		{"shared/tests/support.go", true},
		{"core/router/router.go", false},
	}
	for _, c := range cases {
		if got := inTestDirectory(c.rel); got != c.skip {
			t.Errorf("inTestDirectory(%q) = %v, want %v", c.rel, got, c.skip)
		}
	}
}

func TestTheTestsModuleIsHeldToTheRules(t *testing.T) {
	cases := []struct {
		from string
		to   string
	}{
		{"modules/tests/domain/publish.go", "platform/db"},
		{"modules/tests/application/command/publish.go", "modules/media/repositories"},
		{"modules/tests/repositories/publish_snapshot.go", "modules/media/repositories"},
		{"modules/tests/http/version_management.go", "modules/tests/repositories"},
		{"modules/tests/domain/publish.go", "core/wiring"},
	}
	for _, c := range cases {
		if inTestDirectory(c.from) {
			t.Errorf("%s is skipped as a test directory", c.from)
		}
		if forbidden(c.from, c.to) == "" {
			t.Errorf("%s may import %s", c.from, c.to)
		}
	}
}

func inTestDirectory(rel string) bool {
	parts := strings.Split(rel, "/")
	for i, part := range parts[:len(parts)-1] {
		if part != "tests" {
			continue
		}
		if i == 1 && parts[0] == "modules" {
			continue
		}
		return true
	}
	return false
}

type place struct {
	area   string
	module string
	layer  string
}

func locate(rel string) place {
	parts := strings.Split(rel, "/")
	switch parts[0] {
	case "modules":
		if len(parts) < 3 {
			return place{area: "modules"}
		}
		return place{area: "modules", module: parts[1], layer: parts[2]}
	default:
		return place{area: parts[0]}
	}
}

func forbidden(from, to string) string {
	src, dst := locate(from), locate(to)
	switch src.area {
	case "core":
		return ""
	case "shared":
		if dst.area != "shared" {
			return "shared is the kernel every layer may use; it depends on nothing above it"
		}
	case "platform":
		if dst.area == "modules" || dst.area == "core" {
			return "platform adapters know no module"
		}
	case "modules":
		return forbiddenFromModule(src, dst)
	}
	return ""
}

func forbiddenFromModule(src, dst place) string {
	if dst.area == "core" {
		return "a module never depends on the composition root"
	}
	switch src.layer {
	case "domain":
		if dst.area == "platform" {
			return "domain is pure: no adapter, no framework"
		}
		if dst.area == "modules" && dst.layer != "domain" {
			return "domain may only know another module's domain"
		}
	case "application":
		if dst.area == "platform" {
			return "application reaches infrastructure through ports its own package declares"
		}
		if dst.area == "modules" && (dst.layer == "repositories" || dst.layer == "http") {
			return "application depends on domain and on other modules' domain or application"
		}
	case "repositories":
		if dst.area == "modules" && dst.layer != "domain" {
			return "a repository knows its own domain and other modules' domain types only"
		}
	case "http":
		if dst.area == "modules" && dst.layer == "repositories" {
			return "transport never touches persistence"
		}
	}
	return ""
}
