package domain_test

import (
	"encoding/json"
	"errors"
	"maps"
	"strings"
	"testing"

	"quizzivy/internal/modules/media/domain"

	gen "quizzivy/gen/openapi"
)

func TestLimitsMatchTheContract(t *testing.T) {
	spec, err := gen.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	asset, ok := spec.Components.Schemas["MediaAsset"]
	if !ok || asset.Value == nil {
		t.Fatal("MediaAsset is missing from the spec")
	}

	raw, err := json.Marshal(asset.Value.Extensions["x-media-limits"])
	if err != nil {
		t.Fatal(err)
	}
	var contract map[string]int64
	if err := json.Unmarshal(raw, &contract); err != nil {
		t.Fatalf("MediaAsset's x-media-limits is not a map of numbers: %v", err)
	}
	limits := map[string]int64{
		"audioBytes": domain.MaxAudioBytes,
		"imageBytes": domain.MaxImageBytes,
		"durationMs": int64(domain.MaxDurationMs),
	}
	if !maps.Equal(contract, limits) {
		t.Errorf("the domain's limits %v differ from MediaAsset's x-media-limits %v", limits, contract)
	}

	for _, tc := range []struct {
		field string
		want  int64
	}{
		{"bytes", domain.MaxAudioBytes},
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

func TestTheTransportCapLeavesRoomForTheLargestAudio(t *testing.T) {
	spec, err := gen.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	upload := spec.Paths.Find("/teacher/media").Post
	raw, err := json.Marshal(upload.Extensions["x-max-body-bytes"])
	if err != nil {
		t.Fatal(err)
	}
	var limit int64
	if err := json.Unmarshal(raw, &limit); err != nil {
		t.Fatalf("uploadMedia's x-max-body-bytes is not a number: %v", err)
	}
	if want := domain.MaxAudioBytes + 128<<10; limit != want {
		t.Errorf("uploadMedia lets %d bytes through, want the audio limit and 128 KiB of multipart framing, %d", limit, want)
	}
}

func TestTheNameAndPlayLimitBoundsMatchTheContract(t *testing.T) {
	spec, err := gen.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	for _, schema := range []string{"LibraryAsset", "MediaUpdate"} {
		properties := spec.Components.Schemas[schema].Value.Properties
		name, plays := properties["displayName"].Value, properties["defaultMaxPlays"].Value
		if name.MinLength != 1 || name.MaxLength == nil || *name.MaxLength != domain.MaxDisplayNameLength {
			t.Errorf("%s.displayName allows %d to %v characters, want 1 to %d", schema, name.MinLength, name.MaxLength, domain.MaxDisplayNameLength)
		}
		if plays.Min == nil || *plays.Min != 0 || plays.Max == nil || *plays.Max != domain.MaxDefaultPlays {
			t.Errorf("%s.defaultMaxPlays allows %v to %v, want 0 to %d", schema, plays.Min, plays.Max, domain.MaxDefaultPlays)
		}
	}
	parameter := spec.Paths.Find("/teacher/media").Post.Parameters.GetByInAndName("query", "defaultMaxPlays").Schema.Value
	if parameter.Min == nil || *parameter.Min != 0 || parameter.Max == nil || *parameter.Max != domain.MaxDefaultPlays {
		t.Errorf("uploadMedia's defaultMaxPlays allows %v to %v, want 0 to %d", parameter.Min, parameter.Max, domain.MaxDefaultPlays)
	}
}

func TestASizeIsMeasuredAgainstTheLimitOfItsKind(t *testing.T) {
	for name, c := range map[string]struct {
		kind  domain.Kind
		bytes int64
		want  error
	}{
		"audio at its limit":            {domain.KindAudio, domain.MaxAudioBytes, nil},
		"audio a byte over":             {domain.KindAudio, domain.MaxAudioBytes + 1, domain.ErrTooLarge},
		"an image at its limit":         {domain.KindImage, domain.MaxImageBytes, nil},
		"an image a byte over":          {domain.KindImage, domain.MaxImageBytes + 1, domain.ErrImageTooLarge},
		"audio between the two limits":  {domain.KindAudio, domain.MaxImageBytes + 1, nil},
		"an image over the audio limit": {domain.KindImage, domain.MaxAudioBytes + 1, domain.ErrImageTooLarge},
	} {
		got := domain.Assets.CheckSize(c.kind, c.bytes)
		if !errors.Is(got, c.want) || (c.want == nil && got != nil) {
			t.Errorf("%s: %v, want %v", name, got, c.want)
		}
		if c.want == domain.ErrTooLarge && errors.Is(got, domain.ErrImageTooLarge) {
			t.Errorf("%s is refused as an image is", name)
		}
	}
	if !errors.Is(domain.ErrImageTooLarge, domain.ErrTooLarge) {
		t.Error("an image over its limit is not a file over the limit")
	}
}

func TestAPlayLimitIsZeroToThreeAndForAudioOnly(t *testing.T) {
	limit := func(n int) *int { return &n }
	for name, c := range map[string]struct {
		kind  domain.Kind
		plays *int
		want  error
	}{
		"audio with none":        {domain.KindAudio, nil, nil},
		"an image with none":     {domain.KindImage, nil, nil},
		"audio, unlimited":       {domain.KindAudio, limit(0), nil},
		"audio, three":           {domain.KindAudio, limit(domain.MaxDefaultPlays), nil},
		"audio, four":            {domain.KindAudio, limit(domain.MaxDefaultPlays + 1), domain.ErrInvalidPlayLimit},
		"audio, below zero":      {domain.KindAudio, limit(-1), domain.ErrInvalidPlayLimit},
		"an image, unlimited":    {domain.KindImage, limit(0), domain.ErrPlayLimitOnImage},
		"an image, two":          {domain.KindImage, limit(2), domain.ErrPlayLimitOnImage},
		"an image, out of range": {domain.KindImage, limit(9), domain.ErrInvalidPlayLimit},
	} {
		got := domain.Assets.CheckPlayLimit(c.kind, c.plays)
		if !errors.Is(got, c.want) || (c.want == nil && got != nil) {
			t.Errorf("%s: %v, want %v", name, got, c.want)
		}
	}
}

func TestADisplayNameIsTrimmedAndBounded(t *testing.T) {
	longest := strings.Repeat("ế", domain.MaxDisplayNameLength)
	for name, c := range map[string]struct {
		in, want string
		refused  bool
	}{
		"a plain name":                {"Bài nghe 1.mp3", "Bài nghe 1.mp3", false},
		"spaces around it":            {"  \tBài nghe 1.mp3\n ", "Bài nghe 1.mp3", false},
		"the longest":                 {longest, longest, false},
		"the longest, padded":         {" " + longest + " ", longest, false},
		"one character more":          {longest + "ế", "", true},
		"empty":                       {"", "", true},
		"only spaces":                 {" \t\n ", "", true},
		"only a no-break space":       {" ", "", true},
		"spaces kept inside the name": {"Unit 4  ·  map", "Unit 4  ·  map", false},
	} {
		got, err := domain.Assets.DisplayName(c.in)
		if c.refused != errors.Is(err, domain.ErrInvalidName) || (!c.refused && err != nil) || got != c.want {
			t.Errorf("%s: %q (%v), want %q, refused %v", name, got, err, c.want, c.refused)
		}
	}
}

func TestTheQuotaAllowsALibraryUpToItAndNoFurther(t *testing.T) {
	for name, c := range map[string]struct {
		held, add, quota int64
		refused          bool
	}{
		"an empty library":       {0, 100, 100, false},
		"filling it exactly":     {60, 40, 100, false},
		"one byte past":          {60, 41, 100, true},
		"already full":           {100, 1, 100, true},
		"already past, add none": {101, 0, 100, true},
	} {
		if err := domain.Assets.CheckQuota(c.held, c.add, c.quota); c.refused != errors.Is(err, domain.ErrQuotaExceeded) || (!c.refused && err != nil) {
			t.Errorf("%s: %v, want refused %v", name, err, c.refused)
		}
	}
}
