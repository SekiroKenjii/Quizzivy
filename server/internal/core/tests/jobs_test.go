package core_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func startedBy(t *testing.T, method string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", "core.go"), nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	var started []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name.Name != method {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			spawned, ok := n.(*ast.GoStmt)
			if !ok {
				return true
			}
			if call, ok := spawned.Call.Fun.(*ast.SelectorExpr); ok {
				if pkg, ok := call.X.(*ast.Ident); ok && pkg.Name == "jobs" {
					started = append(started, call.Sel.Name)
				}
			}
			return true
		})
	}
	return started
}

func TestServeStartsTheDailyPrunes(t *testing.T) {
	started := startedBy(t, "Serve")
	for _, job := range []string{"PruneRefreshTokens", "PruneNotifications"} {
		if !slices.Contains(started, job) {
			t.Errorf("Serve starts %v, so %s never runs", started, job)
		}
	}
}

func TestServeStartsTheLegacyJoinCodeRotationOnce(t *testing.T) {
	started := startedBy(t, "Serve")
	n := 0
	for _, job := range started {
		if job == "RotateLegacyJoinCodes" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("Serve starts %v, so the legacy join codes are rotated %d times at start-up, want once", started, n)
	}
}

func TestOnlyTheAPIProcessRotatesLegacyJoinCodes(t *testing.T) {
	worker, err := os.ReadFile(filepath.Join("..", "import_worker.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(worker), "RotateLegacyJoinCodes") {
		t.Error("the import worker starts the legacy join-code rotation; only the API process may")
	}
}
