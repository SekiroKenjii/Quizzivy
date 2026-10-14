package apidocs

import (
	"sort"

	"github.com/getkin/kin-openapi/openapi3"
)

// Summary is what the API reference page says about the contract that only the
// server knows: the versions it is written in, how many operations it declares
// and how many carry each tag.
type Summary struct {
	// OpenAPI is the OpenAPI version of the document.
	OpenAPI string
	// Version is the contract's own info.version.
	Version string
	// EndpointCount is the number of operations, each counted once.
	EndpointCount int
	// Groups lists the tags the contract declares in its order, then any tag an
	// operation carries that it does not declare, in name order.
	Groups []Group
}

// Group is one tag and the number of operations that carry it.
type Group struct {
	Tag   string
	Count int
}

// Summarise reads spec. An operation that names a tag twice counts once under it,
// and one with two tags counts under both.
func Summarise(spec *openapi3.T) Summary {
	out := Summary{OpenAPI: spec.OpenAPI}
	if spec.Info != nil {
		out.Version = spec.Info.Version
	}
	counts := map[string]int{}
	for _, item := range spec.Paths.Map() {
		for _, op := range item.Operations() {
			out.EndpointCount++
			for tag := range uniqueTags(op.Tags) {
				counts[tag]++
			}
		}
	}
	out.Groups = groupsOf(spec.Tags, counts)
	return out
}

func uniqueTags(tags []string) map[string]struct{} {
	unique := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		unique[tag] = struct{}{}
	}
	return unique
}

func groupsOf(declared openapi3.Tags, counts map[string]int) []Group {
	groups := make([]Group, 0, len(declared))
	listed := make(map[string]bool, len(declared))
	for _, tag := range declared {
		listed[tag.Name] = true
		groups = append(groups, Group{Tag: tag.Name, Count: counts[tag.Name]})
	}
	var undeclared []string
	for tag := range counts {
		if !listed[tag] {
			undeclared = append(undeclared, tag)
		}
	}
	sort.Strings(undeclared)
	for _, tag := range undeclared {
		groups = append(groups, Group{Tag: tag, Count: counts[tag]})
	}
	return groups
}
