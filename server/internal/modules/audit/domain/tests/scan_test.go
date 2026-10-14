package domain_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var (
	actionShape    = regexp.MustCompile(`^[a-z][a-z_]*(\.[a-z][a-z_]*){1,2}$`)
	entityShape    = regexp.MustCompile(`^[a-z][a-z_]*$`)
	quotedAction   = regexp.MustCompile(`'([a-z][a-z_]*(?:\.[a-z][a-z_]*){1,2})'`)
	entityAfterAct = regexp.MustCompile(`'[a-z][a-z_]*(?:\.[a-z][a-z_]*){1,2}'\s*,\s*'([a-z][a-z_]*)'`)
	entityAfterEnd = regexp.MustCompile(`\bEND\s*,\s*'([a-z][a-z_]*)'`)
	auditCallee    = regexp.MustCompile(`(?i)audit|record`)
)

type found map[string][]string

func (f found) add(name, where string) { f[name] = append(f[name], where) }

func (f found) names() []string {
	out := make([]string, 0, len(f))
	for name := range f {
		out = append(out, name)
	}
	return out
}

func isAction(s string) bool {
	return actionShape.MatchString(s) && !strings.HasSuffix(s, "_id")
}

func str(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(lit.Value)
	return v, err == nil
}

func calleeName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	}
	return ""
}

func namesAction(name string) bool {
	return strings.EqualFold(name, "action") || strings.HasSuffix(name, "Action")
}

func serverRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "..", "..")
}

func scanServer(t *testing.T) (actions, entities found) {
	t.Helper()
	actions, entities = found{}, found{}
	root := serverRoot(t)
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			if inTestDirectory(path) {
				return nil
			}
			return scanFile(path, actions, entities)
		})
		if err != nil {
			t.Fatalf("scan %s: %v", dir, err)
		}
	}
	return actions, entities
}

func scanFile(path string, actions, entities found) error {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return err
	}
	where := filepath.ToSlash(path)
	constants := packageConstants(filepath.Dir(path))
	writer := writesAudit(file)
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.KeyValueExpr:
			scanKeyValue(n, constants, where, actions, entities)
		case *ast.AssignStmt:
			for i, lhs := range n.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && namesAction(id.Name) && i < len(n.Rhs) {
					addAction(n.Rhs[i], where, actions)
				}
			}
		case *ast.ValueSpec:
			for i, id := range n.Names {
				if namesAction(id.Name) && i < len(n.Values) {
					addAction(n.Values[i], where, actions)
				}
			}
		case *ast.FuncDecl:
			if namesAction(n.Name.Name) && n.Body != nil {
				ast.Inspect(n.Body, func(m ast.Node) bool {
					if ret, ok := m.(*ast.ReturnStmt); ok {
						for _, r := range ret.Results {
							addAction(r, where, actions)
						}
					}
					return true
				})
			}
		case *ast.CallExpr:
			if auditCallee.MatchString(calleeName(n)) {
				for _, arg := range n.Args {
					addAction(arg, where, actions)
				}
			}
		case *ast.BasicLit:
			if v, ok := str(n); ok {
				if strings.Contains(v, "audit_log") {
					scanSQL(v, where, actions, entities)
				}
				if writer && isAction(v) {
					actions.add(v, where)
				}
			}
		}
		return true
	})
	return nil
}

func addAction(e ast.Expr, where string, actions found) {
	if v, ok := str(e); ok && isAction(v) {
		actions.add(v, where)
	}
}

func scanKeyValue(kv *ast.KeyValueExpr, constants map[string]string, where string, actions, entities found) {
	key, ok := kv.Key.(*ast.Ident)
	if !ok {
		return
	}
	switch key.Name {
	case "Action", "action":
		addAction(kv.Value, where, actions)
	case "Entity":
		if v, ok := str(kv.Value); ok {
			entities.add(v, where)
		} else if id, ok := kv.Value.(*ast.Ident); ok {
			if v, ok := constants[id.Name]; ok {
				entities.add(v, where)
			}
		}
	}
}

func scanSQL(sql, where string, actions, entities found) {
	for _, m := range quotedAction.FindAllStringSubmatch(sql, -1) {
		if isAction(m[1]) {
			actions.add(m[1], where)
		}
	}
	for _, re := range []*regexp.Regexp{entityAfterAct, entityAfterEnd} {
		for _, m := range re.FindAllStringSubmatch(sql, -1) {
			if entityShape.MatchString(m[1]) {
				entities.add(m[1], where)
			}
		}
	}
}

func packageConstants(dir string) map[string]string {
	out := map[string]string{}
	files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			continue
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, id := range vs.Names {
					if i < len(vs.Values) {
						if v, ok := str(vs.Values[i]); ok {
							out[id.Name] = v
						}
					}
				}
			}
		}
	}
	return out
}

func inTestDirectory(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i, part := range parts[:len(parts)-1] {
		if part == "tests" && !(i > 0 && parts[i-1] == "modules") {
			return true
		}
	}
	return false
}

const auditPackage = "quizzivy/internal/shared/audit"

func writesAudit(file *ast.File) bool {
	for _, imp := range file.Imports {
		if strings.Trim(imp.Path.Value, `"`) == auditPackage {
			return true
		}
	}
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING && strings.Contains(lit.Value, "audit_log") {
			found = true
		}
		return !found
	})
	return found
}
