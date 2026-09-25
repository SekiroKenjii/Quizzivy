package word_test

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"quizzivy/internal/platform/word"
)

func extracted(t testing.TB, sourceID string, entries []entry) word.Extraction {
	t.Helper()
	data := pack(t, entries)
	got, err := word.Extract(context.Background(), bytes.NewReader(data), int64(len(data)), sourceID, word.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func blockText(b word.SourceBlock) string {
	var text strings.Builder
	if b.Paragraph != nil {
		for _, r := range b.Paragraph.Runs {
			for _, f := range r.Fragments {
				text.WriteString(f.Text)
			}
		}
	}
	return text.String()
}

func TestExtractionOrdersMixedParagraphsTablesAndNestedCells(t *testing.T) {
	body := `<w:p><w:r><w:t>Before</w:t></w:r></w:p><w:tbl><w:tr><w:tc><w:p><w:r><w:t>A</w:t></w:r></w:p><w:tbl><w:tr><w:tc><w:p><w:r><w:t>Nested</w:t></w:r></w:p></w:tc></w:tr></w:tbl><w:p><w:r><w:t>After nested</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>B</w:t></w:r></w:p></w:tc></w:tr></w:tbl><w:p><w:r><w:t>After</w:t></w:r></w:p>`
	got := extracted(t, "source-one", baseEntries(body))
	var texts []string
	byID := map[string]word.SourceBlock{}
	previous := 0
	var cells []word.SourceBlock
	for _, b := range got.Blocks {
		if b.Order <= previous || b.End < b.Order {
			t.Fatalf("non-document order: %+v", b)
		}
		previous = b.Order
		if _, exists := byID[b.ID]; exists {
			t.Fatal("duplicate source identity")
		}
		if b.ParentID != "" {
			parent, exists := byID[b.ParentID]
			if !exists || parent.End < b.End {
				t.Fatal("missing or unrelated parent")
			}
		}
		byID[b.ID] = b
		if b.Paragraph != nil {
			texts = append(texts, blockText(b))
		}
		if b.Cell != nil {
			cells = append(cells, b)
		}
	}
	if !slices.Equal(texts, []string{"Before", "A", "Nested", "After nested", "B", "After"}) {
		t.Fatalf("source reordered: %v", texts)
	}
	if len(cells) != 3 || cells[0].Cell.TableID != cells[2].Cell.TableID || cells[1].Cell.TableID == cells[0].Cell.TableID || cells[2].Cell.Column != 1 {
		t.Fatalf("nested table leaked grid state: %+v", cells)
	}
}

func TestExtractionIdentityUsesImmutableSourceAndNotZipOrder(t *testing.T) {
	entries := baseEntries(`<w:p><w:r><w:t>Duplicate</w:t></w:r></w:p><w:p><w:r><w:t>Duplicate</w:t></w:r></w:p>`)
	a := extracted(t, "source-one", entries)
	slices.Reverse(entries)
	b := extracted(t, "source-one", entries)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("ZIP order changed extraction")
	}
	c := extracted(t, "source-two", entries)
	if a.Blocks[0].ID == a.Blocks[1].ID || a.Blocks[0].ID == c.Blocks[0].ID {
		t.Fatal("source or duplicate text identities collapsed")
	}
}

func TestExtractionOffsetsMarksAndPrivateBranchesAreRetained(t *testing.T) {
	body := `<w:p><w:r><w:rPr><w:u w:val="single"/></w:rPr><w:t>Chữ 🙂</w:t></w:r><w:r><w:rPr><w:vanish/></w:rPr><w:t>secret</w:t></w:r><w:del><w:r><w:delText>old</w:delText></w:r></w:del></w:p>`
	got := extracted(t, "source-one", baseEntries(body))
	p := got.Blocks[0]
	if p.Paragraph == nil || len(p.Paragraph.Runs) != 3 {
		t.Fatal("source branch lost")
	}
	runs := p.Paragraph.Runs
	if runs[0].Fragments[0].EndOffset != 5 || runs[1].Fragments[0].Start != 5 || runs[2].Fragments[0].EndOffset != 14 {
		t.Fatalf("offsets are not Unicode code points: %+v", runs)
	}
	if !slices.ContainsFunc(runs[0].Marks, func(m word.ResolvedMark) bool { return m.Name == "u" && m.Resolved && m.Value == "single" }) {
		t.Fatal("meaningful underline lost")
	}
	if !slices.Contains(p.ReviewReasons, "HIDDEN_TEXT_REQUIRES_REVIEW") || !slices.Contains(p.ReviewReasons, "TRACKED_CHANGE_REQUIRES_REVIEW") {
		t.Fatalf("private branches appeared ordinary: %v", p.ReviewReasons)
	}
	if blockText(p) != "Chữ 🙂secretold" {
		t.Fatal("private source was silently discarded")
	}
}

func TestExtractionKeepsAutomaticLabelsOutOfRunOffsets(t *testing.T) {
	got := extracted(t, "source-one", resolutionEntries(numbered(1, 0, "First")+numbered(1, 0, "Second"), "", simpleNumbering(level(0, 1, "decimal", "%1.", ""))))
	var labels []string
	for _, b := range got.Blocks {
		if b.Paragraph == nil || b.Paragraph.Numbering == nil {
			continue
		}
		labels = append(labels, b.Paragraph.Numbering.Text)
		if b.Paragraph.Runs[0].Fragments[0].Start != 0 {
			t.Fatal("generated labels changed source offsets")
		}
	}
	if !slices.Equal(labels, []string{"1.", "2."}) {
		t.Fatalf("automatic labels lost: %v", labels)
	}
}

func TestExtractionMarksComplexFieldsAcrossParagraphs(t *testing.T) {
	body := `<w:p><w:r><w:t>Before</w:t></w:r></w:p><w:p><w:r><w:fldChar w:fldCharType="begin"/><w:instrText>PRIVATE</w:instrText></w:r></w:p><w:p><w:r><w:fldChar w:fldCharType="separate"/><w:t>Result</w:t></w:r></w:p><w:p><w:r><w:fldChar w:fldCharType="end"/></w:r></w:p><w:p><w:r><w:t>After</w:t></w:r></w:p>`
	got := extracted(t, "source-one", baseEntries(body))
	for _, b := range got.Blocks {
		if b.Paragraph == nil {
			continue
		}
		want := blockText(b) != "Before" && blockText(b) != "After"
		if slices.Contains(b.ReviewReasons, "FIELD_REQUIRES_REVIEW") != want {
			t.Fatalf("field boundary lost for %q: %v", blockText(b), b.ReviewReasons)
		}
	}
}

func TestExtractionResolvesMergedCellOriginsAndFlagsOrphans(t *testing.T) {
	body := `<w:tbl><w:tr><w:tc><w:tcPr><w:gridSpan w:val="2"/><w:vMerge w:val="restart"/></w:tcPr><w:p><w:r><w:t>Shared</w:t></w:r></w:p></w:tc></w:tr><w:tr><w:tc><w:tcPr><w:gridSpan w:val="2"/><w:vMerge/></w:tcPr><w:p/></w:tc></w:tr><w:tr><w:tc><w:tcPr><w:gridSpan w:val="1"/><w:vMerge/></w:tcPr><w:p/></w:tc></w:tr></w:tbl>`
	got := extracted(t, "source-one", baseEntries(body))
	var cells []word.SourceBlock
	for _, b := range got.Blocks {
		if b.Cell != nil {
			cells = append(cells, b)
		}
	}
	if len(cells) != 3 || !cells[0].Cell.Resolved || !cells[1].Cell.Resolved || cells[1].Cell.MergeOriginID != cells[0].ID || cells[2].Cell.Resolved || len(cells[2].ReviewReasons) == 0 {
		t.Fatalf("incorrect merge inference: %+v", cells)
	}
}

func TestExtractionDoesNotExecuteOrChooseAlternateObjects(t *testing.T) {
	body := `<mc:AlternateContent xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"><mc:Choice Requires="w"><w:p><w:r><w:t>Primary</w:t></w:r></w:p></mc:Choice><mc:Fallback><w:p><w:r><w:t>Fallback</w:t></w:r></w:p></mc:Fallback></mc:AlternateContent>`
	got := extracted(t, "source-one", baseEntries(body))
	var texts []string
	for _, b := range got.Blocks {
		if b.Paragraph != nil {
			texts = append(texts, blockText(b))
			if !slices.Contains(b.ReviewReasons, "OBJECT_REQUIRES_REVIEW") {
				t.Fatal("branch was implicitly accepted")
			}
		}
	}
	if !slices.Equal(texts, []string{"Primary", "Fallback"}) {
		t.Fatal("source alternative disappeared")
	}
}

func TestExtractionBoundsInputAndHonorsCancellation(t *testing.T) {
	data := pack(t, baseEntries(`<w:p/>`))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := word.Extract(ctx, bytes.NewReader(data), int64(len(data)), "source", word.DefaultLimits()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel ignored: %v", err)
	}
	if _, err := word.Extract(context.Background(), bytes.NewReader(data), int64(len(data)), "", word.DefaultLimits()); !errors.Is(err, word.ErrInvalidPackage) {
		t.Fatalf("source identity ignored: %v", err)
	}
}

func TestExtractionFlagsInlineInsertedTextAndSimpleFieldResults(t *testing.T) {
	body := `<w:p><w:ins><w:r><w:t>Inserted</w:t></w:r></w:ins></w:p><w:p><w:fldSimple w:instr="PRIVATE"><w:r><w:t>Result</w:t></w:r></w:fldSimple></w:p><w:p><w:r><w:rPr><w:rPrChange><w:rPr><w:b/></w:rPr></w:rPrChange></w:rPr><w:t>Changed</w:t></w:r></w:p>`
	got := extracted(t, "source-one", baseEntries(body))
	for _, b := range got.Blocks {
		if b.Paragraph == nil {
			continue
		}
		want := "TRACKED_CHANGE_REQUIRES_REVIEW"
		if blockText(b) == "Result" {
			want = "FIELD_REQUIRES_REVIEW"
		}
		if !slices.Contains(b.ReviewReasons, want) {
			t.Fatalf("inline source ambiguity lost for %q", blockText(b))
		}
	}
}

func TestExtractionDoesNotGuessDuplicateOrOverflowingCellProperties(t *testing.T) {
	for _, props := range []string{`<w:gridSpan w:val="1"/><w:gridSpan w:val="2"/>`, `<w:gridSpan w:val="9223372036854775807"/>`, `<w:vMerge w:val="restart"/><w:vMerge/>`, `<w:hMerge/>`} {
		got := extracted(t, "source-one", baseEntries(`<w:tbl><w:tr><w:tc><w:tcPr>`+props+`</w:tcPr><w:p/></w:tc></w:tr></w:tbl>`))
		for _, b := range got.Blocks {
			if b.Cell != nil && b.Cell.Resolved {
				t.Fatal("ambiguous cell accepted")
			}
		}
	}
}

func FuzzExtractionPreservesDistinctBoundedSourceIdentities(f *testing.F) {
	f.Add(pack(f, baseEntries(`<w:p><w:r><w:t>Example</w:t></w:r></w:p>`)))
	f.Add(pack(f, baseEntries(`<w:tbl><w:tr><w:tc><w:p/></w:tc></w:tr></w:tbl>`)))
	f.Fuzz(func(t *testing.T, data []byte) {
		limits := word.DefaultLimits()
		limits.CompressedBytes = 1 << 20
		limits.ExpandedBytes = 2 << 20
		limits.XMLTotalBytes = 2 << 20
		limits.XMLNodes = 10000
		limits.LocatorBytes = 1 << 20
		got, err := word.Extract(context.Background(), bytes.NewReader(data), int64(len(data)), "fuzz-source", limits)
		if err != nil {
			return
		}
		seen := map[string]word.SourceBlock{}
		for _, b := range got.Blocks {
			if _, exists := seen[b.ID]; exists {
				t.Fatal("duplicate identity")
			}
			if b.ParentID != "" {
				parent, exists := seen[b.ParentID]
				if !exists || parent.Part != b.Part || parent.Order > b.Order || parent.End < b.End {
					t.Fatal("invalid source parent")
				}
			}
			seen[b.ID] = b
		}
	})
}
