package domain_test

import (
	"testing"

	"quizzivy/internal/modules/media/domain"

	gen "quizzivy/gen/openapi"
)

// TestLimitsMatchTheContract pins §11.1's two numbers against the embedded
// spec.
func TestLimitsMatchTheContract(t *testing.T) {
	spec, err := gen.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	asset, ok := spec.Components.Schemas["MediaAsset"]
	if !ok || asset.Value == nil {
		t.Fatal("MediaAsset is missing from the spec")
	}

	for _, tc := range []struct {
		field string
		want  int64
	}{
		{"bytes", domain.MaxBytes},
		{"durationMs", int64(domain.MaxDurationMs)},
	} {
		property, ok := asset.Value.Properties[tc.field]
		if !ok || property.Value == nil {
			t.Errorf("MediaAsset.%s is missing from the spec", tc.field)
			continue
		}
		if property.Value.Max == nil {
			t.Errorf("MediaAsset.%s declares no maximum, so nothing pins the Go constant", tc.field)
			continue
		}
		if got := int64(*property.Value.Max); got != tc.want {
			t.Errorf("MediaAsset.%s maximum is %d, but the Go constant is %d", tc.field, got, tc.want)
		}
	}
}
