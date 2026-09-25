package content_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"quizzivy/internal/shared/content"
)

type fixture struct {
	Name      string          `json:"name"`
	Document  json.RawMessage `json:"document"`
	Valid     bool            `json:"valid"`
	PlainText string          `json:"plainText"`
	Gaps      []string        `json:"gaps"`
	Assets    []struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
	} `json:"assets"`
}

func fixtures(t testing.TB) []fixture {
	t.Helper()
	raw, err := os.ReadFile("../../../../../api/testdata/content-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []fixture
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

func TestSharedContractCases(t *testing.T) {
	for _, tc := range fixtures(t) {
		t.Run(tc.Name, func(t *testing.T) {
			doc, err := content.Parse(tc.Document)
			if !tc.Valid {
				if !errors.Is(err, content.ErrInvalidDocument) {
					t.Fatalf("want rejection, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if doc.PlainText() != tc.PlainText {
				t.Fatalf("projection: %q, want %q", doc.PlainText(), tc.PlainText)
			}
			if !slices.Equal(doc.GapIDs(), tc.Gaps) {
				t.Fatalf("gap IDs: %v", doc.GapIDs())
			}
			assets := doc.Assets()
			if len(assets) != len(tc.Assets) {
				t.Fatalf("assets: %v", assets)
			}
			for i, asset := range assets {
				if asset.ID != tc.Assets[i].ID || asset.Kind != tc.Assets[i].Kind {
					t.Fatalf("asset %d: %v", i, asset)
				}
			}
			assertRoundTrip(t, doc)
		})
	}
}

func assertRoundTrip(t testing.TB, doc content.Document) {
	t.Helper()
	raw, err := doc.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	again, err := content.Parse(raw)
	if err != nil || again.PlainText() != doc.PlainText() || again.Format() != doc.Format() || !slices.Equal(again.GapIDs(), doc.GapIDs()) || !slices.Equal(again.Assets(), doc.Assets()) {
		t.Fatalf("round trip changed content: %v", err)
	}
}

func TestStrictJSON(t *testing.T) {
	for _, raw := range []string{
		`{"format":"legacy_markdown_v1","markdown":"a","markdown":"b"}`,
		`{"format":"legacy_markdown_v1","markdown":"a","mark\u0064own":"b"}`,
		`{"format":"legacy_markdown_v1","markdown":"a"} {}`,
		`{"format":"legacy_markdown_v1","markdown":"\ud800a"}`,
		`{"format":"legacy_markdown_v1","markdown":"\ud800\u1234"}`,
		`{"format":"legacy_markdown_v1","markdown":"\udfff"}`,
		`{"format":"legacy_markdown_v1","markdown":"` + string([]byte{0xff}) + `"}`,
		`{"format":"legacy_markdown_v1","markdown":NaN}`,
		`{"format":"legacy_markdown_v1","markdown":"\u0000"}`,
	} {
		if _, err := content.Parse([]byte(raw)); !errors.Is(err, content.ErrInvalidDocument) {
			t.Errorf("expected strict rejection: %v", err)
		}
	}
	for _, raw := range []string{
		`{"format":"legacy_markdown_v1","markdown":"\ud83c\udf31"}`,
		`{"format":"legacy_markdown_v1","markdown":"\\ud800"}`,
		`{"format":"legacy_markdown_v1","markdown":"�"}`,
	} {
		if _, err := content.Parse([]byte(raw)); err != nil {
			t.Errorf("valid Unicode rejected: %v", err)
		}
	}
}

func TestDocumentDoesNotExposeMutableStorage(t *testing.T) {
	for _, tc := range fixtures(t) {
		if !tc.Valid {
			continue
		}
		doc, err := content.Parse(tc.Document)
		if err != nil {
			t.Fatal(err)
		}
		before, err := doc.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		clear(tc.Document)
		if gaps := doc.GapIDs(); len(gaps) > 0 {
			gaps[0] = "changed"
		}
		if assets := doc.Assets(); len(assets) > 0 {
			assets[0].ID = "changed"
		}
		mutated, err := doc.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		clear(mutated)
		after, err := doc.MarshalJSON()
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("document mutated through public accessor")
		}
		assertRoundTrip(t, doc)
	}
	if _, err := json.Marshal(content.Document{}); !errors.Is(err, content.ErrInvalidDocument) {
		t.Fatal("zero document should not serialize")
	}
}

func legacy(t testing.TB, text string) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]string{"format": "legacy_markdown_v1", "markdown": text})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestBudgets(t *testing.T) {
	paragraph := `{"type":"paragraph","content":[{"type":"text","text":"x","marks":[]}]}`
	boundary := `{"format":"semantic_v1","blocks":[` + strings.Repeat(paragraph+",", 1023) + paragraph + `]}`
	if _, err := content.Parse([]byte(boundary)); err != nil {
		t.Fatal("node boundary", err)
	}
	over := `{"format":"semantic_v1","blocks":[` + strings.Repeat(paragraph+",", 1024) + paragraph + `]}`
	if _, err := content.Parse([]byte(over)); err == nil {
		t.Fatal("aggregate node limit ignored")
	}
	textNode := `{"type":"paragraph","content":[{"type":"text","text":"` + strings.Repeat("x", 50001) + `","marks":[]}]}`
	overText := `{"format":"semantic_v1","blocks":[` + textNode + "," + textNode + `]}`
	if _, err := content.Parse([]byte(overText)); err == nil {
		t.Fatal("aggregate visible text limit ignored")
	}

	if _, err := content.Parse(legacy(t, strings.Repeat("🌱", content.MaxText))); err != nil {
		t.Fatal("scalar text boundary", err)
	}
	tooMuch := legacy(t, strings.Repeat("🌱", content.MaxText+1))
	deep := []byte(strings.Repeat("[", content.MaxDepth+2) + "0" + strings.Repeat("]", content.MaxDepth+2))
	wide := []byte("[" + strings.Repeat("0,", content.MaxNodes) + "0]")
	bytesOver := append(legacy(t, "valid"), bytes.Repeat([]byte{' '}, content.MaxBytes)...)
	for _, raw := range [][]byte{tooMuch, deep, wide, bytesOver} {
		if _, err := content.Parse(raw); !errors.Is(err, content.ErrInvalidDocument) {
			t.Fatalf("over-budget input accepted: %v", err)
		}
	}
}

func FuzzParse(f *testing.F) {
	for _, tc := range fixtures(f) {
		f.Add([]byte(tc.Document))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		doc, err := content.Parse(raw)
		if err != nil {
			if !errors.Is(err, content.ErrInvalidDocument) {
				t.Fatal("unexpected error", err)
			}
			return
		}
		assertRoundTrip(t, doc)
	})
}
