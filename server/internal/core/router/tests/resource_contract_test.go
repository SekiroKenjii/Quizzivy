package router_test

import (
	"encoding/json"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

var resourceKinds = []string{
	"test", "test-version", "section", "question", "version-question", "version-recording", "question-group", "media",
	"import", "import-source", "class", "assignment", "attempt", "student", "none",
}

type resourceEntry struct {
	In   string `json:"in"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

func scopedTree(path string) bool {
	return strings.HasPrefix(path, "/teacher/") || strings.HasPrefix(path, "/app/") || strings.HasPrefix(path, "/me/")
}

func uuidPointers(ref *openapi3.SchemaRef, at string, seen map[*openapi3.Schema]bool, out map[string]bool) {
	if ref == nil || ref.Value == nil || seen[ref.Value] {
		return
	}
	s := ref.Value
	if s.Format == "uuid" {
		out[at] = true
		return
	}
	seen[s] = true
	defer delete(seen, s)
	uuidPointers(s.Items, at+"/-", seen, out)
	for name, prop := range s.Properties {
		uuidPointers(prop, at+"/"+strings.NewReplacer("~", "~0", "/", "~1").Replace(name), seen, out)
	}
	if names := s.PropertyNames; names != nil && names.Value != nil && names.Value.Format == "uuid" {
		out[at+"/+"] = true
	}
	uuidPointers(s.AdditionalProperties.Schema, at+"/*", seen, out)
	for _, group := range []openapi3.SchemaRefs{s.AllOf, s.OneOf, s.AnyOf} {
		for _, sub := range group {
			uuidPointers(sub, at, seen, out)
		}
	}
}

func uuidFields(item *openapi3.PathItem, op *openapi3.Operation) []string {
	fields := map[string]bool{}
	for _, params := range []openapi3.Parameters{item.Parameters, op.Parameters} {
		for _, p := range params {
			found := map[string]bool{}
			uuidPointers(p.Value.Schema, "", map[*openapi3.Schema]bool{}, found)
			if len(found) > 0 {
				fields[p.Value.In+" "+p.Value.Name] = true
			}
		}
	}
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		for _, media := range op.RequestBody.Value.Content {
			found := map[string]bool{}
			uuidPointers(media.Schema, "", map[*openapi3.Schema]bool{}, found)
			for pointer := range found {
				fields["body "+pointer] = true
			}
		}
	}
	out := make([]string, 0, len(fields))
	for f := range fields {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

func declaredResources(t *testing.T, pattern string, op *openapi3.Operation) []resourceEntry {
	t.Helper()
	raw, ok := op.Extensions["x-resource"]
	if !ok {
		return nil
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	var entries []resourceEntry
	if err := json.Unmarshal(encoded, &entries); err != nil {
		t.Fatalf("%s: x-resource is not a list of {in, name, kind}: %v", pattern, err)
	}
	return entries
}

func TestEveryScopedUUIDNamesItsResourceKind(t *testing.T) {
	spec := freshSpec(t)
	checked := 0
	for path, item := range spec.Paths.Map() {
		if !scopedTree(path) {
			continue
		}
		for method, op := range item.Operations() {
			pattern := method + " " + path
			want := uuidFields(item, op)
			entries := declaredResources(t, pattern, op)
			var got []string
			for _, e := range entries {
				got = append(got, e.In+" "+e.Name)
				if !slices.Contains(resourceKinds, e.Kind) {
					t.Errorf("%s: %s %s names unknown kind %q", pattern, e.In, e.Name, e.Kind)
				}
				checked++
			}
			sort.Strings(got)
			if !slices.Equal(got, want) {
				t.Errorf("%s: x-resource covers %v, the uuid fields are %v", pattern, got, want)
			}
		}
	}
	if checked < 100 {
		t.Errorf("only %d x-resource entries were checked; the walk is not reading what it thinks it is", checked)
	}
}

func listsItems(ref *openapi3.SchemaRef) bool {
	if ref == nil || ref.Value == nil {
		return false
	}
	if items := ref.Value.Properties["items"]; items != nil && items.Value != nil && items.Value.Type.Is("array") {
		return true
	}
	return slices.ContainsFunc(ref.Value.AllOf, listsItems)
}

func TestEveryTeacherListNamesTheKindItLists(t *testing.T) {
	spec := freshSpec(t)
	lists := 0
	for path, item := range spec.Paths.Map() {
		if !strings.HasPrefix(path, "/teacher/") || item.Get == nil {
			continue
		}
		op := item.Get
		response := op.Responses.Status(200)
		if response == nil || response.Value == nil {
			continue
		}
		media := response.Value.Content.Get("application/json")
		if media == nil || media.Schema == nil || media.Schema.Value == nil {
			continue
		}
		kind, declared := op.Extensions["x-resource-list"].(string)
		isList := listsItems(media.Schema)
		if isList {
			lists++
		}
		switch {
		case isList && !declared:
			t.Errorf("GET %s lists items but declares no x-resource-list", path)
		case !isList && declared:
			t.Errorf("GET %s declares x-resource-list but lists no items", path)
		case declared && (kind == "none" || !slices.Contains(resourceKinds, kind)):
			t.Errorf("GET %s lists unknown kind %q", path, kind)
		}
	}
	if lists != 12 {
		t.Errorf("%d teacher lists, want 12", lists)
	}
}
