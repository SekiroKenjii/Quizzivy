package core_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
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

func TestServeStartsTheMediaSweepOnceAndOnlyWhileItsSwitchIsOn(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", "core.go"), nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	starts, guarded := 0, 0
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name.Name != "Serve" {
			continue
		}
		var enclosing []ast.Node
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if n == nil {
				enclosing = enclosing[:len(enclosing)-1]
				return true
			}
			enclosing = append(enclosing, n)
			spawned, ok := n.(*ast.GoStmt)
			if !ok {
				return true
			}
			call, ok := spawned.Call.Fun.(*ast.SelectorExpr)
			if !ok || call.Sel.Name != "SweepRetiredMedia" {
				return true
			}
			starts++
			for _, outer := range enclosing {
				if branch, ok := outer.(*ast.IfStmt); ok && mentionsField(branch.Cond, "MediaRetentionSweep") {
					guarded++
				}
			}
			return true
		})
	}
	if starts != 1 || guarded != 1 {
		t.Errorf("Serve starts the media sweep %d times, %d of them under MediaRetentionSweep, want once and guarded", starts, guarded)
	}
}

func mentionsField(expr ast.Expr, name string) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if field, ok := n.(*ast.SelectorExpr); ok && field.Sel.Name == name {
			found = true
		}
		return !found
	})
	return found
}
