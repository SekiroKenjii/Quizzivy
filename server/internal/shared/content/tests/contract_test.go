package content_test

import (
	"encoding/json"
	"maps"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"quizzivy/internal/shared/content"
)

func TestLimitsMatchOpenAPI(t *testing.T) {
	spec, err := openapi3.NewLoader().LoadFromFile("../../../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(spec.Components.Schemas["ContentDocument"].Value.Extensions["x-content-limits"])
	if err != nil {
		t.Fatal(err)
	}
	var contract map[string]int
	if err := json.Unmarshal(raw, &contract); err != nil {
		t.Fatal(err)
	}
	limits := map[string]int{
		"bytes": content.MaxBytes, "nodes": content.MaxNodes, "depth": content.MaxDepth,
		"values": content.MaxValues, "strings": content.MaxStrings, "text": content.MaxText,
		"url": content.MaxURL, "rows": content.MaxRows, "columns": content.MaxColumns,
	}
	if !maps.Equal(contract, limits) {
		t.Fatalf("domain limits %v differ from OpenAPI %v", limits, contract)
	}
}
