package apidocs_test

import (
	"slices"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"quizzivy/gen/openapi"
	"quizzivy/internal/platform/apidocs"
)

func operationOn(tags ...string) *openapi3.Operation {
	return &openapi3.Operation{Tags: tags}
}

func TestSummariseCountsOperationsAndTagsInDeclaredOrder(t *testing.T) {
	paths := openapi3.NewPaths()
	paths.Set("/a", &openapi3.PathItem{
		Get:  operationOn("beta"),
		Post: operationOn("alpha", "beta"),
	})
	paths.Set("/b", &openapi3.PathItem{Get: operationOn("beta", "beta"), Delete: operationOn("stray")})
	spec := &openapi3.T{
		OpenAPI: "3.1.0",
		Info:    &openapi3.Info{Version: "7.2.1"},
		Paths:   paths,
		Tags:    openapi3.Tags{{Name: "unused"}, {Name: "beta"}, {Name: "alpha"}},
	}

	got := apidocs.Summarise(spec)

	if got.OpenAPI != "3.1.0" || got.Version != "7.2.1" || got.EndpointCount != 4 {
		t.Errorf("summary %+v, want openapi 3.1.0, version 7.2.1 and 4 operations", got)
	}
	want := []apidocs.Group{{Tag: "unused", Count: 0}, {Tag: "beta", Count: 3}, {Tag: "alpha", Count: 1}, {Tag: "stray", Count: 1}}
	if !slices.Equal(got.Groups, want) {
		t.Errorf("groups %v, want %v: declared tags in order, a tag nobody declared last", got.Groups, want)
	}
}

func TestSummariseOfAnEmptyContractHasNoGroupsAndNoOperations(t *testing.T) {
	got := apidocs.Summarise(&openapi3.T{OpenAPI: "3.1.0", Paths: openapi3.NewPaths()})
	if got.EndpointCount != 0 || got.Groups == nil || len(got.Groups) != 0 || got.Version != "" {
		t.Errorf("summary %+v, want an empty one whose groups are an empty list", got)
	}
}

func TestTheEmbeddedContractIsSummarisedFaithfully(t *testing.T) {
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatalf("GetSpec: %v", err)
	}
	got := apidocs.Summarise(spec)

	operations := 0
	carried := map[string]int{}
	for _, item := range spec.Paths.Map() {
		for _, op := range item.Operations() {
			operations++
			for _, tag := range slices.Compact(slices.Sorted(slices.Values(op.Tags))) {
				carried[tag]++
			}
		}
	}
	if got.EndpointCount != operations || operations == 0 {
		t.Errorf("endpoint count %d, the spec declares %d operations", got.EndpointCount, operations)
	}
	if got.OpenAPI != spec.OpenAPI || got.Version != spec.Info.Version {
		t.Errorf("versions %q / %q, the spec says %q / %q", got.OpenAPI, got.Version, spec.OpenAPI, spec.Info.Version)
	}
	seen := map[string]bool{}
	for _, g := range got.Groups {
		seen[g.Tag] = true
		if g.Count != carried[g.Tag] {
			t.Errorf("group %s counts %d, %d operations carry it", g.Tag, g.Count, carried[g.Tag])
		}
	}
	for tag := range carried {
		if !seen[tag] {
			t.Errorf("tag %s is carried by operations and has no group", tag)
		}
	}
	for _, tag := range spec.Tags {
		if !seen[tag.Name] {
			t.Errorf("declared tag %s has no group", tag.Name)
		}
	}
}

func TestEveryTagInTheContractHasADescription(t *testing.T) {
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatalf("GetSpec: %v", err)
	}
	declared := map[string]bool{}
	for _, tag := range spec.Tags {
		declared[tag.Name] = true
		if tag.Description == "" {
			t.Errorf("tag %s has no description", tag.Name)
		}
	}
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			for _, tag := range op.Tags {
				if !declared[tag] {
					t.Errorf("%s %s carries the tag %s, which the contract does not declare", method, path, tag)
				}
			}
		}
	}
}
