package router_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"testing"
)

// unimplemented is every operation that still returns 501, listed on purpose.
var unimplemented = []string{}

// stubbedOperations reads the names off router/server.go, which by convention
// holds nothing but stubs: implementing an operation moves its method out.
func stubbedOperations(t *testing.T) []string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath.Join("..", "server.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse server.go: %v", err)
	}

	var names []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 {
			continue
		}
		star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		ident, ok := star.X.(*ast.Ident)
		if !ok || ident.Name != "Server" {
			continue
		}
		names = append(names, fn.Name.Name)
	}
	slices.Sort(names)
	return names
}

func TestTheListOfUnimplementedOperationsIsAccurate(t *testing.T) {
	found := stubbedOperations(t)

	want := slices.Clone(unimplemented)
	slices.Sort(want)

	if !slices.Equal(found, want) {
		missing := difference(want, found)
		extra := difference(found, want)
		if len(missing) > 0 {
			t.Errorf("listed as unimplemented but no longer stubbed (delete from the list): %v", missing)
		}
		if len(extra) > 0 {
			t.Errorf("still a stub but not listed (add it, and check no screen depends on it): %v", extra)
		}
	}
}

func difference(a, b []string) []string {
	var out []string
	for _, s := range a {
		if !slices.Contains(b, s) {
			out = append(out, s)
		}
	}
	return out
}
