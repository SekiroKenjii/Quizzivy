package core_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

func configFieldsPassedTo(t *testing.T, file, function string) map[string][]string {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", "wiring", file), nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	passed := map[string][]string{}
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != function {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			method, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			fields := []string{}
			for _, arg := range call.Args {
				ast.Inspect(arg, func(n ast.Node) bool {
					if field, ok := n.(*ast.SelectorExpr); ok {
						if owner, ok := field.X.(*ast.Ident); ok && (owner.Name == "cfg" || owner.Name == "adapters") {
							fields = append(fields, owner.Name+"."+field.Sel.Name)
						}
					}
					return true
				})
			}
			passed[method.Sel.Name] = fields
			return true
		})
	}
	return passed
}

func TestTheMediaApplicationIsBuiltWithItsProbesAndItsQuota(t *testing.T) {
	passed := configFieldsPassedTo(t, "media.go", "media")
	for option, want := range map[string]string{
		"WithImageProbe":   "adapters.ImageProbe",
		"WithOwnerQuota":   "cfg.MediaOwnerQuotaMiB",
		"WithSignedURLTTL": "cfg.SignedURLTTL",
	} {
		fields, called := passed[option]
		if !called || len(fields) != 1 || fields[0] != want {
			t.Errorf("wiring builds the media application with %s given %v (called %v), want it given %s", option, fields, called, want)
		}
	}
}
