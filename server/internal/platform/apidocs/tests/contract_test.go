package apidocs_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"quizzivy/gen/contract"
	"quizzivy/internal/platform/apidocs"
)

func contractSource(t *testing.T) []byte {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func converted(t *testing.T, source string) string {
	t.Helper()
	out, err := apidocs.ContractJSON([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func compact(t *testing.T, document string) string {
	t.Helper()
	var out bytes.Buffer
	if err := json.Compact(&out, []byte(document)); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestContractJSONKeepsTheOrderTheKeysWereWrittenIn(t *testing.T) {
	got := compact(t, converted(t, "zebra: 1\nalpha:\n  - b: 1\n    a: 2\nmiddle: x\n"))
	if want := `{"zebra":1,"alpha":[{"b":1,"a":2}],"middle":"x"}`; got != want {
		t.Errorf("converted to %s, want %s", got, want)
	}
}

func TestContractJSONWritesEveryScalarAsItsJSONKind(t *testing.T) {
	source := "text: hello\nquoted: '123'\nyes: on\nflag: true\nnothing: null\nempty:\nint: 42\nneg: -7\nfloat: 0.10\nexp: 1e3\n" +
		"hex: 0x1F\nplus: +5\ndate: 2026-10-14\nmulti: |\n  line one\n  line two\nunicode: Nguyễn Văn A\nescaped: \"tab\\there \\\"quoted\\\"\"\n"
	got := compact(t, converted(t, source))
	want := `{"text":"hello","quoted":"123","yes":"on","flag":true,"nothing":null,"empty":null,"int":42,"neg":-7,"float":0.10,"exp":1e3,` +
		`"hex":31,"plus":5,"date":"2026-10-14","multi":"line one\nline two\n","unicode":"Nguyễn Văn A","escaped":"tab\there \"quoted\""}`
	if got != want {
		t.Errorf("converted to\n%s\nwant\n%s", got, want)
	}
}

func TestContractJSONNamesANumericKeyAsTheStringItIsWrittenAs(t *testing.T) {
	got := compact(t, converted(t, "responses:\n  200:\n    description: ok\n  '404':\n    description: gone\n  default:\n    description: other\n"))
	want := `{"responses":{"200":{"description":"ok"},"404":{"description":"gone"},"default":{"description":"other"}}}`
	if got != want {
		t.Errorf("converted to %s, want %s", got, want)
	}
}

func TestContractJSONRefusesWhatJSONCannotSayTheSameWayTwice(t *testing.T) {
	for name, source := range map[string]string{
		"a repeated key":            "a: 1\nb: 2\na: 3\n",
		"an alias":                  "base: &b {x: 1}\nuse: *b\n",
		"a merge key":               "base: &b {x: 1}\nuse:\n  <<: *b\n  y: 2\n",
		"a mapping used as a key":   "? {a: 1}\n: v\n",
		"two documents":             "a: 1\n---\nb: 2\n",
		"nothing":                   "",
		"text that is not YAML":     "a: [1, 2\n",
		"a binary scalar":           "blob: !!binary aGVsbG8=\n",
		"a number JSON cannot hold": "n: .inf\n",
	} {
		if out, err := apidocs.ContractJSON([]byte(source)); err == nil {
			t.Errorf("%s converted to %s, want an error", name, out)
		}
	}
}

func TestContractJSONIsIndentedAndEndsWithANewline(t *testing.T) {
	got := converted(t, "a:\n  b: 1\n")
	if want := "{\n  \"a\": {\n    \"b\": 1\n  }\n}\n"; got != want {
		t.Errorf("converted to %q, want %q", got, want)
	}
}

var httpMethods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

var operationID = regexp.MustCompile(`(?m)^\s+operationId:\s*['"]?([A-Za-z0-9_]+)['"]?\s*$`)

func TestTheDownloadedContractIsTheAuthoredOneNotTheGeneratorsRewrite(t *testing.T) {
	var document struct {
		OpenAPI string `json:"openapi"`
		Info    struct {
			Version string `json:"version"`
		} `json:"info"`
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(contract.JSON(), &document); err != nil {
		t.Fatal(err)
	}
	var authored []string
	for _, m := range operationID.FindAllSubmatch(contractSource(t), -1) {
		authored = append(authored, string(m[1]))
	}
	var served []string
	for _, item := range document.Paths {
		for method, raw := range item {
			var op struct {
				OperationID string `json:"operationId"`
			}
			if slices.Contains(httpMethods, method) {
				if err := json.Unmarshal(raw, &op); err != nil {
					t.Fatal(err)
				}
				served = append(served, op.OperationID)
			}
		}
	}
	slices.Sort(authored)
	slices.Sort(served)
	if !slices.Equal(authored, served) || len(served) < 100 {
		t.Errorf("%d operation ids are served and %d are written in api/openapi.yaml, or they differ: login should be %q, not %q",
			len(served), len(authored), "login", "Login")
	}
	if !slices.Contains(served, "login") || slices.Contains(served, "Login") {
		t.Error("the downloaded contract does not name the sign-in operation login")
	}
	if document.OpenAPI == "" || document.Info.Version == "" {
		t.Errorf("openapi %q, info.version %q: the document's head is missing", document.OpenAPI, document.Info.Version)
	}
}

func TestTheCommittedContractIsWhatTheSourceConvertsTo(t *testing.T) {
	want, err := apidocs.ContractJSON(contractSource(t))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(contract.JSON(), want) {
		t.Error("server/gen/contract/openapi.json differs from api/openapi.yaml: run make gen and commit the result")
	}
	if !strings.HasPrefix(string(want), "{\n  \"openapi\": ") {
		t.Errorf("the converted contract starts %q, want the openapi key first as written", want[:30])
	}
}

var valueKeywords = map[string]bool{"default": true, "example": true, "examples": true, "const": true}

func nullKeys(t *testing.T, document []byte) []string {
	t.Helper()
	var root any
	if err := json.Unmarshal(document, &root); err != nil {
		t.Fatal(err)
	}
	var found []string
	var walk func(path string, node any)
	walk = func(path string, node any) {
		switch v := node.(type) {
		case map[string]any:
			for key, child := range v {
				if valueKeywords[key] {
					continue
				}
				at := path + "." + key
				if child == nil {
					found = append(found, at)
					continue
				}
				walk(at, child)
			}
		case []any:
			for i, child := range v {
				walk(fmt.Sprintf("%s[%d]", path, i), child)
			}
		}
	}
	walk("$", root)
	slices.Sort(found)
	return found
}

func TestANullValuedKeyIsFoundExceptWhereTheKeywordHoldsAValue(t *testing.T) {
	document := converted(t, "properties:\n  left: { type: string, description: Pause a teacher, or let them ask for more. }\n"+
		"  kept: { type: string, default: null, example: { a: null }, const: null, enum: [x, null] }\n"+
		"  quoted: { type: string, description: \"Pause a teacher, or let them ask for more.\" }\n")
	got := nullKeys(t, []byte(document))
	if want := []string{"$.properties.left.or let them ask for more."}; !slices.Equal(got, want) {
		t.Errorf("found null-valued keys %q, want %q", got, want)
	}
}

func TestTheDownloadedContractHasNoNullValuedKey(t *testing.T) {
	if got := nullKeys(t, contract.JSON()); len(got) > 0 {
		t.Errorf("api/openapi.yaml has %d keys with no value, which a description cut at a comma in a flow mapping leaves behind; quote the description: %q", len(got), got)
	}
}

func TestTheDownloadedContractIsAValidOpenAPIDocument(t *testing.T) {
	document, err := openapi3.NewLoader().LoadFromData(contract.JSON())
	if err != nil {
		t.Fatalf("the downloaded contract does not load: %v", err)
	}
	if err := document.Validate(context.Background()); err != nil {
		t.Errorf("the downloaded contract is not a valid OpenAPI document: %v", err)
	}
	if document.Paths.Len() < 100 {
		t.Errorf("the downloaded contract has %d paths", document.Paths.Len())
	}
}
