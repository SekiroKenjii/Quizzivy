package media_test

import (
	"testing"

	gen "quizzivy/gen/openapi"
	"quizzivy/internal/media"
)

// TestLimitsMatchTheContract pins §11.1's two numbers against the embedded
// spec.
//
// They are written out in four places -- here, the contract, the migration's
// CHECK, and the web client -- because none of the four can derive them from
// another. Drift in the direction someone would naturally take, raising the
// user-facing check first, means the upload runs, the object lands in R2, and
// the INSERT fails on the CHECK: the expensive work happens and then the write
// is refused, which is the shape of #15.
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
		{"bytes", media.MaxBytes},
		{"durationMs", int64(media.MaxDurationMs)},
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
