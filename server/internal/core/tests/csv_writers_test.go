package core_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const tabularPackage = "platform/tabular/"

func TestOnlyTabularWritesCSV(t *testing.T) {
	var offences []string
	for _, root := range []struct{ dir, prefix string }{{filepath.Join("..", ".."), ""}, {filepath.Join("..", "..", "..", "cmd"), "cmd/"}} {
		err := filepath.WalkDir(root.dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel := root.prefix + filepath.ToSlash(strings.TrimPrefix(path, root.dir+string(filepath.Separator)))
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || inTestDirectory(rel) || strings.HasPrefix(rel, tabularPackage) {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			lines, err := csvWriterLines(src)
			if err != nil {
				return err
			}
			for _, line := range lines {
				offences = append(offences, rel+":"+strconv.Itoa(line))
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(offences)
	for _, o := range offences {
		t.Errorf("%s builds a csv.Writer: a table that leaves the product goes through tabular.NewCSV, which adds the byte-order mark and escapes every cell", o)
	}
}

func TestTheCSVWriterGuardSeesTheCallsItIsMeantToCatch(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []int
	}{
		{"a writer", "package p\nimport \"encoding/csv\"\nvar _ = csv.NewWriter(nil)\n", []int{3}},
		{"an aliased import", "package p\nimport c \"encoding/csv\"\nfunc f() { _ = c.NewWriter(nil) }\n", []int{3}},
		{"a reader", "package p\nimport \"encoding/csv\"\nvar _ = csv.NewReader(nil)\n", nil},
		{"another package's NewWriter", "package p\nimport \"bufio\"\nvar _ = bufio.NewWriter(nil)\n", nil},
		{"a method of that name without the import", "package p\ntype w struct{}\nfunc (w) NewWriter() {}\nfunc f() { var csv w; csv.NewWriter() }\n", nil},
	}
	for _, c := range cases {
		got, err := csvWriterLines([]byte(c.src))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: lines = %v, want %v", c.name, got, c.want)
		}
	}
}

func csvWriterLines(src []byte) ([]int, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	local := ""
	for _, imp := range file.Imports {
		if strings.Trim(imp.Path.Value, `"`) != "encoding/csv" {
			continue
		}
		local = "csv"
		if imp.Name != nil {
			local = imp.Name.Name
		}
	}
	if local == "" {
		return nil, nil
	}
	var lines []int
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "NewWriter" {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == local {
			lines = append(lines, fset.Position(call.Pos()).Line)
		}
		return true
	})
	return lines, nil
}
