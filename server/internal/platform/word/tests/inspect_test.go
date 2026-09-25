package word_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"quizzivy/internal/platform/word"
)

const (
	wNS       = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	rNS       = "http://schemas.openxmlformats.org/package/2006/relationships"
	ctNS      = "http://schemas.openxmlformats.org/package/2006/content-types"
	officeRel = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"
	mainType  = "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"
)

type entry struct {
	name string
	text string
}

func document(body string) string {
	return `<w:document xmlns:w="` + wNS + `"><w:body>` + body + `</w:body></w:document>`
}

func baseEntries(body string) []entry {
	return []entry{
		{"[Content_Types].xml", `<Types xmlns="` + ctNS + `"><Default Extension="xml" ContentType="application/xml"/><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Override PartName="/word/document.xml" ContentType="` + mainType + `"/></Types>`},
		{"_rels/.rels", `<Relationships xmlns="` + rNS + `"><Relationship Id="main" Type="` + officeRel + `" Target="word/document.xml"/></Relationships>`},
		{"word/document.xml", document(body)},
	}
}

func pack(t testing.TB, entries []entry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	zw := zip.NewWriter(&buffer)
	for _, e := range entries {
		writer, err := zw.Create(e.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(e.text)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func inspect(t testing.TB, entries []entry) word.Inspection {
	t.Helper()
	data := pack(t, entries)
	got, err := word.Inspect(context.Background(), bytes.NewReader(data), int64(len(data)), word.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func main(t testing.TB, got word.Inspection) word.Part {
	t.Helper()
	for _, part := range got.Parts {
		if part.Name == got.MainPart {
			return part
		}
	}
	t.Fatal("main part missing")
	return word.Part{}
}

func finding(got word.Inspection, code string) bool {
	return slices.ContainsFunc(got.Findings, func(f word.Finding) bool { return f.Code == code })
}

func TestCorpusPreservesSourceTextAndFlagsUnresolvedSemantics(t *testing.T) {
	data, err := os.ReadFile("testdata/corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus []struct {
		ID         string   `json:"id"`
		Source     string   `json:"source"`
		Paragraphs []string `json:"paragraphs"`
		Findings   []string `json:"findings"`
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range corpus {
		t.Run(fixture.ID, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join("testdata", fixture.Source))
			if err != nil {
				t.Fatal(err)
			}
			got := inspect(t, baseEntries(string(source)))
			var paragraphs []string
			for _, p := range main(t, got).Paragraphs {
				var text strings.Builder
				for _, run := range p.Runs {
					text.WriteString(run.Text)
				}
				paragraphs = append(paragraphs, text.String())
			}
			if !reflect.DeepEqual(paragraphs, fixture.Paragraphs) {
				t.Errorf("paragraphs: want %q, got %q", fixture.Paragraphs, paragraphs)
			}
			for _, code := range fixture.Findings {
				if !finding(got, code) {
					t.Errorf("missing required finding %s", code)
				}
			}
		})
	}
}

func TestMeaningfulRunFormattingAndTableCoordinatesRemainSourceEvidence(t *testing.T) {
	got := inspect(t, baseEntries(`<w:tbl><w:tr><w:tc><w:tcPr><w:gridSpan w:val="2"/></w:tcPr><w:p><w:r><w:rPr><w:u w:val="single"/><w:b/><w:i/><w:vertAlign w:val="superscript"/><w:color w:val="FF0000"/></w:rPr><w:t>ou</w:t></w:r></w:p></w:tc></w:tr></w:tbl>`))
	p := main(t, got).Paragraphs[0]
	if len(p.Containers) != 3 || len(p.Runs) != 1 || len(p.Runs[0].Properties) != 5 {
		t.Fatalf("lost table or formatting evidence: %+v", p)
	}
	if p.Runs[0].Properties[0].Attributes["{"+wNS+"}val"] != "single" {
		t.Fatal("underline meaning lost")
	}
	cell := main(t, got).Structures[2]
	if len(cell.Properties) != 1 || cell.Properties[0].Children[0].Attributes["{"+wNS+"}val"] != "2" {
		t.Fatal("merged table cell evidence lost")
	}
	if !strings.Contains(p.Containers[2].Path, "}tc[1]") || !strings.Contains(p.Runs[0].Path, "}r[1]") {
		t.Fatal("table cell/run locator absent")
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("isCorrect")) || bytes.Contains(encoded, []byte("confidence")) {
		t.Fatal("inspection must not infer an answer or confidence from formatting")
	}
}

func TestLocatorsAndPartOrderDoNotDependOnZipEntryOrder(t *testing.T) {
	entries := baseEntries(`<w:p><w:r><w:t>One</w:t></w:r><w:r><w:t> two</w:t></w:r></w:p><w:p><w:r><w:t>Three</w:t></w:r></w:p>`)
	entries = append(entries, entry{"word/_rels/document.xml.rels", `<Relationships xmlns="` + rNS + `"><Relationship Id="link" Type="hyperlink" Target="https://example.com" TargetMode="External"/></Relationships>`})
	a := inspect(t, entries)
	slices.Reverse(entries)
	b := inspect(t, entries)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("same source changed when ZIP entries were reordered")
	}
	p := main(t, a).Paragraphs
	if p[0].Path == p[1].Path || p[0].Runs[0].Path == p[0].Runs[1].Path {
		t.Fatal("distinct source locations have the same identity")
	}
}

func TestDrawingRelationshipAndGeometryRemainSourceEvidence(t *testing.T) {
	entries := baseEntries(`<w:p><w:r><w:drawing><wp:inline xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"><wp:extent cx="100" cy="200"/><a:blip xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:embed="image1"/></wp:inline></w:drawing></w:r></w:p>`)
	entries[0].text = strings.Replace(entries[0].text, "</Types>", `<Default Extension="png" ContentType="image/png"/></Types>`, 1)
	entries = append(entries, entry{"word/media/image.png", "not decoded by the inventory"}, entry{"word/_rels/document.xml.rels", `<Relationships xmlns="` + rNS + `"><Relationship Id="image1" Type="image" Target="media/image.png"/></Relationships>`})
	got := inspect(t, entries)
	objects := main(t, got).Objects
	if len(objects) != 1 || len(got.Assets) != 1 || !finding(got, "DRAWING_REQUIRES_RESOLUTION") {
		t.Fatal("drawing evidence not retained")
	}
	children := objects[0].Content.Children[0].Children
	if children[0].Attributes["{}cx"] != "100" || children[1].Attributes["{http://schemas.openxmlformats.org/officeDocument/2006/relationships}embed"] != "image1" {
		t.Fatal("image relationship or placement lost")
	}
}

func TestAlternateTextBranchesStayDistinctUntilResolved(t *testing.T) {
	got := inspect(t, baseEntries(`<mc:AlternateContent xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"><mc:Choice Requires="w"><w:p><w:r><w:t>Primary</w:t></w:r></w:p></mc:Choice><mc:Fallback><w:p><w:r><w:t>Fallback</w:t></w:r></w:p></mc:Fallback></mc:AlternateContent>`))
	part := main(t, got)
	if !finding(got, "ALTERNATE_CONTENT_REQUIRES_RESOLUTION") || len(part.Objects) != 1 || len(part.Paragraphs) != 2 {
		t.Fatal("alternate evidence must not silently choose or merge a branch")
	}
	if !strings.Contains(part.Paragraphs[0].Path, "}Choice[1]") || !strings.Contains(part.Paragraphs[1].Path, "}Fallback[1]") {
		t.Fatal("alternate branch locators are ambiguous")
	}
}

func TestFieldInstructionsStaySeparateFromDisplayedText(t *testing.T) {
	got := inspect(t, baseEntries(`<w:p><w:r><w:instrText>PAGE</w:instrText><w:t>7</w:t></w:r></w:p>`))
	run := main(t, got).Paragraphs[0].Runs[0]
	if len(run.Fragments) != 2 || run.Fragments[0].Kind != "instrText" || run.Fragments[1].Kind != "t" {
		t.Fatal("field code cannot become indistinguishable from displayed content")
	}
}

func TestUnresolvedNumberingAndStylePartsAreRetainedWithoutParagraphs(t *testing.T) {
	entries := baseEntries(`<w:p/>`)
	entries = append(entries, entry{"word/numbering.xml", `<w:numbering xmlns:w="` + wNS + `"><w:abstractNum w:abstractNumId="0"><w:lvl w:ilvl="0"><w:start w:val="1"/><w:lvlText w:val="%1."/></w:lvl></w:abstractNum></w:numbering>`})
	got := inspect(t, entries)
	idx := slices.IndexFunc(got.Parts, func(p word.Part) bool { return p.Name == "word/numbering.xml" })
	if idx < 0 || len(got.Parts[idx].Properties) != 1 || !finding(got, "UNSUPPORTED_SOURCE_PART") {
		t.Fatal("unclassified XML cannot be silently discarded")
	}
}

func TestStrictWordNamespaceAndNonstandardMainPart(t *testing.T) {
	entries := baseEntries(`<w:p><w:r><w:t>Strict</w:t></w:r></w:p>`)
	for i := range entries {
		entries[i].name = strings.ReplaceAll(entries[i].name, "word/document.xml", "paper/exam.xml")
		entries[i].text = strings.ReplaceAll(entries[i].text, "word/document.xml", "paper/exam.xml")
		entries[i].text = strings.ReplaceAll(entries[i].text, wNS, "http://purl.oclc.org/ooxml/wordprocessingml/main")
	}
	got := inspect(t, entries)
	if got.MainPart != "paper/exam.xml" || main(t, got).Paragraphs[0].Runs[0].Text != "Strict" {
		t.Fatalf("main relationship was not followed: %+v", got)
	}
}

func TestHeadersFootnotesAndStyleDefinitionsAreInventoried(t *testing.T) {
	entries := baseEntries(`<w:p><w:pPr><w:pStyle w:val="Question"/></w:pPr><w:r><w:t>Body</w:t></w:r></w:p>`)
	entries = append(entries,
		entry{"word/header1.xml", `<w:hdr xmlns:w="` + wNS + `"><w:p><w:r><w:t>Teacher instructions</w:t></w:r></w:p></w:hdr>`},
		entry{"word/footnotes.xml", `<w:footnotes xmlns:w="` + wNS + `"><w:footnote w:id="2"><w:p><w:r><w:t>Required context</w:t></w:r></w:p></w:footnote></w:footnotes>`},
		entry{"word/styles.xml", `<w:styles xmlns:w="` + wNS + `"><w:style w:styleId="Question"><w:basedOn w:val="Normal"/><w:rPr><w:u w:val="single"/></w:rPr></w:style></w:styles>`})
	for _, kind := range []string{"header", "footnotes", "styles"} {
		name := kind
		if kind == "header" {
			name = "header1"
		}
		decl := `<Override PartName="/word/` + name + `.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.` + kind + `+xml"/>`
		entries[0].text = strings.Replace(entries[0].text, "</Types>", decl+"</Types>", 1)
	}
	got := inspect(t, entries)
	if len(got.Parts) != 4 || !finding(got, "STYLE_RESOLUTION_REQUIRED") {
		t.Fatalf("source parts were not inventoried: %+v", got)
	}
	style := slices.IndexFunc(got.Parts, func(p word.Part) bool { return p.Kind == "styles" })
	if style < 0 || len(got.Parts[style].Properties) == 0 {
		t.Fatal("inherited formatting evidence lost")
	}
}

func TestExternalRelationshipsAreRecordedWithoutNetworkAccess(t *testing.T) {
	entries := baseEntries(`<w:p><w:r><w:t>External diagram</w:t></w:r></w:p>`)
	entries = append(entries, entry{"word/_rels/document.xml.rels", `<Relationships xmlns="` + rNS + `"><Relationship Id="remote" Type="image" Target="http://localhost:1/private" TargetMode="External"/></Relationships>`})
	got := inspect(t, entries)
	if len(got.Relationships) != 2 || !finding(got, "EXTERNAL_RELATIONSHIP") {
		t.Fatal("external dependency must remain visible for semantic review")
	}
}

func TestInternalRelationshipRetainsItsFragmentAndOriginalSpelling(t *testing.T) {
	entries := baseEntries(`<w:p/>`)
	entries = append(entries, entry{"word/_rels/document.xml.rels", `<Relationships xmlns="` + rNS + `"><Relationship Id="bookmark" Type="hyperlink" Target="#section-2"/></Relationships>`})
	got := inspect(t, entries)
	idx := slices.IndexFunc(got.Relationships, func(r word.Relationship) bool { return r.ID == "bookmark" })
	if idx < 0 || got.Relationships[idx].Target != "word/document.xml" || got.Relationships[idx].OriginalTarget != "#section-2" {
		t.Fatal("internal link's source fragment disappeared during resolution")
	}
}

func TestUnassignedTextIsRetainedAndCannotPassAsAnEmptySuccess(t *testing.T) {
	got := inspect(t, baseEntries(`<w:t>Outside a run</w:t>`))
	if !finding(got, "UNASSIGNED_SOURCE_TEXT") || len(main(t, got).Unassigned) != 1 {
		t.Fatal("unrecognized text disappeared")
	}
	if !finding(got, "NO_EXTRACTABLE_TEXT") {
		t.Fatal("no usable paragraphs must not masquerade as a recognized exam")
	}
}

func TestCancelledInspectionDoesNotStart(t *testing.T) {
	data := pack(t, baseEntries(`<w:p/>`))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := word.Inspect(ctx, bytes.NewReader(data), int64(len(data)), word.DefaultLimits())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want cancellation, got %v", err)
	}
}

func BenchmarkInspectFiftyQuestionSource(b *testing.B) {
	var body strings.Builder
	for i := range 50 {
		fmt.Fprintf(&body, `<w:p><w:r><w:t>Question %d. Choose the correct answer.</w:t></w:r></w:p><w:p><w:r><w:rPr><w:u w:val="single"/></w:rPr><w:t>A. One B. Two C. Three D. Four</w:t></w:r></w:p>`, i+1)
	}
	data := pack(b, baseEntries(body.String()))
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for b.Loop() {
		if _, err := word.Inspect(context.Background(), bytes.NewReader(data), int64(len(data)), word.DefaultLimits()); err != nil {
			b.Fatal(err)
		}
	}
}

func TestAnUnmatchedFieldEndDoesNotMarkEarlierParagraphs(t *testing.T) {
	body := `<w:p><w:r><w:t>Question 1</w:t></w:r></w:p><w:p><w:r><w:fldChar w:fldCharType="end"/></w:r><w:r><w:t>Question 2</w:t></w:r></w:p>`
	data := pack(t, baseEntries(body))
	got, err := word.Extract(context.Background(), bytes.NewReader(data), int64(len(data)), "sha256:x", word.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range got.Blocks {
		if b.Kind == "paragraph" && b.Paragraph != nil && slices.Contains(b.ReviewReasons, "FIELD_REQUIRES_REVIEW") && strings.Contains(paragraphText(b), "Question 1") {
			t.Fatal("paragraph before an unmatched field end marked as a field")
		}
	}
}

func paragraphText(b word.SourceBlock) string {
	var out strings.Builder
	for _, r := range b.Paragraph.Runs {
		for _, f := range r.Fragments {
			out.WriteString(f.Text)
		}
	}
	return out.String()
}
