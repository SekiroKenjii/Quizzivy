package core_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func transports(t *testing.T, server string) []string {
	t.Helper()
	modules, err := filepath.Glob(filepath.Join(server, "internal", "modules", "*", "http"))
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) < 9 {
		t.Fatalf("internal/modules/*/http matches %d directories, want the module transports, at least 9: %v", len(modules), modules)
	}
	dirs := make([]string, 0, len(modules)+3)
	for _, module := range modules {
		dirs = append(dirs, filepath.ToSlash(strings.TrimPrefix(module, server+string(filepath.Separator))))
	}
	return append(dirs, "internal/platform/httpx", "internal/platform/httpapi", "internal/core/router")
}

func TestNoTransportMessageIsVietnameseOnly(t *testing.T) {
	server := filepath.Join("..", "..", "..")
	checkMessages(t, server, transports(t, server))
}

// The modules whose domain, use cases and repositories word every field
// message in Vietnamese and in English (D-009, #284). A module joins the list
// once its messages are pairs; the questions module went first, in R6.
var pairedModules = []string{"questions"}

func TestNoDomainMessageIsVietnameseOnly(t *testing.T) {
	server := filepath.Join("..", "..", "..")
	var dirs []string
	for _, module := range pairedModules {
		for _, layer := range []string{"domain", "application", "repositories"} {
			dirs = append(dirs, "internal/modules/"+module+"/"+layer)
		}
	}
	checkMessages(t, server, dirs)
}

func checkMessages(t *testing.T, server string, dirs []string) {
	t.Helper()
	var offenders []string
	for _, dir := range dirs {
		checked := 0
		err := filepath.WalkDir(filepath.Join(server, filepath.FromSlash(dir)), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			checked++
			rel := filepath.ToSlash(strings.TrimPrefix(path, server+string(filepath.Separator)))
			found, err := vietnameseOnly(path)
			for _, offence := range found {
				offenders = append(offenders, rel+":"+offence)
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		if checked == 0 {
			t.Errorf("%s holds no Go file, so nothing in it was checked", dir)
		}
	}
	for _, o := range offenders {
		t.Error(o)
	}
}

func vietnameseOnly(path string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	worded := map[*ast.BasicLit]bool{}
	var offences []string
	report := func(pos token.Pos, what string) {
		offences = append(offences, fmt.Sprintf("%d: %s", fset.Position(pos).Line, what))
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if !wordsAMessage(node, file.Name.Name) {
				return true
			}
			for i, arg := range node.Args {
				if lit := sentence(arg); lit != nil && (i == 1 || i == 2) {
					worded[lit] = true
				}
			}
			if len(node.Args) < 3 || isBlank(node.Args[2]) {
				report(node.Pos(), "this message has no English sentence")
			}
		case *ast.BasicLit:
			if node.Kind == token.STRING && !worded[node] && beyondASCII(node.Value) {
				report(node.Pos(), "this sentence is not the Vietnamese or English argument of httpx.Text, httpx.TextFor, AddIn, Refuse or Pair")
			}
		}
		return true
	})
	return offences, nil
}

// wordsAMessage reports whether call takes a message as its second argument in
// Vietnamese and its third in English: httpx.Text and httpx.TextFor after the
// context, and validation's AddIn, Refuse and Pair after the field.
func wordsAMessage(call *ast.CallExpr, pkg string) bool {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		if fun.Sel.Name == "AddIn" || fun.Sel.Name == "Refuse" || fun.Sel.Name == "Pair" {
			return true
		}
		qualifier, ok := fun.X.(*ast.Ident)
		return ok && qualifier.Name == "httpx" && (fun.Sel.Name == "Text" || fun.Sel.Name == "TextFor")
	case *ast.Ident:
		return pkg == "httpx" && (fun.Name == "Text" || fun.Name == "TextFor")
	}
	return false
}

// sentence is the string literal arg words a message with: the literal itself,
// or the format of a fmt.Sprintf that fills it in. Nil otherwise.
func sentence(arg ast.Expr) *ast.BasicLit {
	if call, ok := arg.(*ast.CallExpr); ok && len(call.Args) > 0 {
		fun, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || fun.Sel.Name != "Sprintf" {
			return nil
		}
		if qualifier, ok := fun.X.(*ast.Ident); !ok || qualifier.Name != "fmt" {
			return nil
		}
		arg = call.Args[0]
	}
	lit, ok := arg.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return nil
	}
	return lit
}

func isBlank(arg ast.Expr) bool {
	lit := sentence(arg)
	if lit == nil {
		return false
	}
	return strings.TrimSpace(unquoted(lit.Value)) == ""
}

func beyondASCII(literal string) bool {
	for _, r := range unquoted(literal) {
		if r > 0x7F {
			return true
		}
	}
	return false
}

func unquoted(literal string) string {
	if value, err := strconv.Unquote(literal); err == nil {
		return value
	}
	return literal
}
