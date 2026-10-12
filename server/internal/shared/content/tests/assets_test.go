package content_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"quizzivy/internal/shared/content"
	"strings"
	"testing"
)

const originalAsset = "01935000-0000-7000-8000-000000000001"
const nextAsset = "01935000-0000-7000-8000-000000000002"

func TestAssetReplacementCopiesOnlySemanticAssetIdentities(t *testing.T) {
	raw := []byte(`{"format":"semantic_v1","blocks":[{"type":"image","assetId":"01935000-0000-7000-8000-000000000001","alt":"01935000-0000-7000-8000-000000000001"},{"type":"paragraph","content":[{"type":"text","text":"Giữ nguyên","marks":["bold"]},{"type":"gap","id":"gap-1","label":"Điền"}]}]}`)
	d, err := content.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := d.MarshalJSON()
	copy, err := d.WithAssetIDs(map[string]string{strings.ToUpper(originalAsset): nextAsset})
	if err != nil {
		t.Fatal(err)
	}
	after, _ := copy.MarshalJSON()
	unchanged, _ := d.MarshalJSON()
	if !bytes.Equal(before, unchanged) {
		t.Fatal("source document mutated")
	}
	var actual, expected any
	if err := json.Unmarshal(after, &actual); err != nil {
		t.Fatal(err)
	}
	expectedRaw := bytes.Replace(raw, []byte(`"assetId":"`+originalAsset+`"`), []byte(`"assetId":"`+nextAsset+`"`), 1)
	if err := json.Unmarshal(expectedRaw, &expected); err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(actual)
	b, _ := json.Marshal(expected)
	if !bytes.Equal(a, b) {
		t.Fatalf("unrelated content changed: %s", after)
	}
	if got := copy.Assets(); len(got) != 1 || got[0].ID != nextAsset || got[0].Kind != "image" {
		t.Fatalf("assets=%v", got)
	}
}

func TestAssetReplacementCanonicalizesAudioAndImageMatching(t *testing.T) {
	for _, kind := range []string{"audio", "image"} {
		t.Run(kind, func(t *testing.T) {
			field := "label"
			if kind == "image" {
				field = "alt"
			}
			old := "019350ab-0000-7000-8000-000000000001"
			d, err := content.Parse([]byte(`{"format":"semantic_v1","blocks":[{"type":"` + kind + `","assetId":"` + strings.ToUpper(old) + `","` + field + `":"Ngữ liệu"}]}`))
			if err != nil {
				t.Fatal(err)
			}
			copy, err := d.WithAssetIDs(map[string]string{old: nextAsset})
			if err != nil {
				t.Fatal(err)
			}
			if got := copy.Assets(); len(got) != 1 || got[0].ID != nextAsset {
				t.Fatalf("canonical match=%v", got)
			}
		})
	}
}

func TestAssetReplacementRejectsInvalidMappingAndRevalidatesKinds(t *testing.T) {
	d, err := content.Parse([]byte(`{"format":"semantic_v1","blocks":[{"type":"image","assetId":"` + originalAsset + `","alt":"Hình"},{"type":"audio","assetId":"` + nextAsset + `","label":"Nghe"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, mapping := range []map[string]string{{"invalid": nextAsset}, {originalAsset: "invalid"}, {originalAsset: nextAsset}} {
		if _, err := d.WithAssetIDs(mapping); !errors.Is(err, content.ErrInvalidDocument) {
			t.Fatalf("mapping=%v error=%v", mapping, err)
		}
	}
}

func TestAssetReplacementPreservesLegacyAndUnmatchedDocuments(t *testing.T) {
	for _, raw := range []string{`{"format":"legacy_markdown_v1","markdown":"assetId ` + originalAsset + `"}`, `{"format":"semantic_v1","blocks":[{"type":"image","assetId":"` + nextAsset + `","alt":"Hình"}]}`} {
		d, err := content.Parse([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		copy, err := d.WithAssetIDs(map[string]string{originalAsset: nextAsset})
		if err != nil {
			t.Fatal(err)
		}
		before, _ := d.MarshalJSON()
		after, _ := copy.MarshalJSON()
		var a, b any
		if err := json.Unmarshal(before, &a); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(after, &b); err != nil {
			t.Fatal(err)
		}
		aa, _ := json.Marshal(a)
		bb, _ := json.Marshal(b)
		if !bytes.Equal(aa, bb) {
			t.Fatalf("unmatched document changed: %s", after)
		}
	}
}
