package word_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"quizzivy/internal/platform/word"
)

func resolutionEntries(body, styles, numbering string) []entry {
	entries := baseEntries(body)
	rels := `<Relationships xmlns="` + rNS + `">`
	for _, definition := range []struct{ kind, body string }{{"styles", styles}, {"numbering", numbering}} {
		if definition.body == "" {
			continue
		}
		name := "word/" + definition.kind + ".xml"
		entries[0].text = strings.Replace(entries[0].text, "</Types>", `<Override PartName="/`+name+`" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.`+definition.kind+`+xml"/></Types>`, 1)
		entries = append(entries, entry{name, `<w:` + definition.kind + ` xmlns:w="` + wNS + `">` + definition.body + `</w:` + definition.kind + `>`})
		rels += `<Relationship Id="` + definition.kind + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/` + definition.kind + `" Target="` + definition.kind + `.xml"/>`
	}
	entries = append(entries, entry{"word/_rels/document.xml.rels", rels + `</Relationships>`})
	return entries
}

func resolved(t testing.TB, entries []entry) word.Resolution {
	t.Helper()
	got, err := word.Resolve(context.Background(), inspect(t, entries))
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func mark(t testing.TB, run word.ResolvedRun, name string) word.ResolvedMark {
	t.Helper()
	for _, mark := range run.Marks {
		if mark.Name == name {
			return mark
		}
	}
	t.Fatalf("mark %q absent: %+v", name, run)
	return word.ResolvedMark{}
}

func TestResolutionInheritsSemanticMarksAndKeepsProvenance(t *testing.T) {
	styles := `<w:docDefaults><w:rPrDefault><w:rPr><w:i/></w:rPr></w:rPrDefault></w:docDefaults>
	<w:style w:type="paragraph" w:styleId="Base"><w:rPr><w:u w:val="single"/><w:b/></w:rPr></w:style>
	<w:style w:type="paragraph" w:styleId="Normal" w:default="1"><w:basedOn w:val="Base"/><w:rPr><w:vertAlign w:val="superscript"/></w:rPr></w:style>
	<w:style w:type="character" w:styleId="Clear"><w:rPr><w:u w:val="none"/></w:rPr></w:style>`
	body := `<w:p><w:pPr><w:rPr><w:vanish/></w:rPr></w:pPr><w:r><w:t>Inherited</w:t></w:r><w:r><w:rPr><w:rStyle w:val="Clear"/><w:b w:val="0"/><w:i w:val="0"/><w:vertAlign w:val="baseline"/></w:rPr><w:t>Direct</w:t></w:r></w:p>`
	source := inspect(t, resolutionEntries(body, styles, ""))
	before := fmt.Sprintf("%#v", source)
	got, err := word.Resolve(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if before != fmt.Sprintf("%#v", source) {
		t.Fatal("resolution mutated raw source")
	}
	runs := got.Paragraphs[0].Runs
	for name, want := range map[string]string{"i": "on", "b": "on", "u": "single", "vertAlign": "superscript"} {
		m := mark(t, runs[0], name)
		if !m.Resolved || m.Value != want || len(m.Sources) != 1 || m.Sources[0].Part != "word/styles.xml" || m.Sources[0].Path == "" {
			t.Fatalf("%s: %+v", name, m)
		}
	}
	for name, want := range map[string]string{"i": "off", "b": "off", "u": "none", "vertAlign": "baseline"} {
		m := mark(t, runs[1], name)
		if !m.Resolved || m.Value != want || len(m.Sources) != 2 {
			t.Fatalf("%s: %+v", name, m)
		}
	}
	if len(runs[0].Marks) != 4 || !runs[0].Complete || !runs[1].Complete {
		t.Fatalf("paragraph mark leaked or inheritance failed: %+v", runs)
	}
}

func TestResolutionDoesNotGuessStyleTogglesOrInvalidChains(t *testing.T) {
	for _, tc := range []struct{ name, styles, body, code string }{
		{"cycle", `<w:style w:type="paragraph" w:styleId="a"><w:basedOn w:val="b"/><w:rPr><w:u w:val="single"/></w:rPr></w:style><w:style w:type="paragraph" w:styleId="b"><w:basedOn w:val="a"/></w:style>`, `<w:pPr><w:pStyle w:val="a"/></w:pPr>`, "STYLE_CHAIN_REQUIRES_REVIEW"},
		{"missing", `<w:style w:type="paragraph" w:styleId="a"><w:basedOn w:val="absent"/></w:style>`, `<w:pPr><w:pStyle w:val="a"/></w:pPr>`, "STYLE_CHAIN_REQUIRES_REVIEW"},
		{"toggle", `<w:style w:type="paragraph" w:styleId="a"><w:rPr><w:b/></w:rPr></w:style><w:style w:type="character" w:styleId="b"><w:rPr><w:b/></w:rPr></w:style>`, `<w:pPr><w:pStyle w:val="a"/></w:pPr>`, "SEMANTIC_MARK_REQUIRES_REVIEW"},
		{"duplicate", `<w:style w:type="paragraph" w:styleId="a"/><w:style w:type="paragraph" w:styleId="a"/>`, `<w:pPr><w:pStyle w:val="a"/></w:pPr>`, "STYLE_CHAIN_REQUIRES_REVIEW"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := `<w:p>` + tc.body + `<w:r><w:rPr><w:rStyle w:val="b"/><w:u w:val="single"/></w:rPr><w:t>Text</w:t></w:r></w:p>`
			if tc.name != "toggle" {
				body = strings.Replace(body, `<w:rStyle w:val="b"/>`, "", 1)
			}
			got := resolved(t, resolutionEntries(body, tc.styles, ""))
			if got.Paragraphs[0].Runs[0].Complete || !slices.ContainsFunc(got.Findings, func(f word.Finding) bool { return f.Code == tc.code }) {
				t.Fatalf("ambiguity suppressed: %+v", got)
			}
			for _, m := range got.Paragraphs[0].Runs[0].Marks {
				if !m.Resolved && m.Value != "" {
					t.Fatalf("guessed value: %+v", m)
				}
			}
		})
	}
}

func level(index, start int, format, pattern, extra string) string {
	return fmt.Sprintf(`<w:lvl w:ilvl="%d"><w:start w:val="%d"/><w:numFmt w:val="%s"/><w:lvlText w:val="%s"/>%s</w:lvl>`, index, start, format, pattern, extra)
}

func numbered(id, index int, text string) string {
	return fmt.Sprintf(`<w:p><w:pPr><w:numPr><w:ilvl w:val="%d"/><w:numId w:val="%d"/></w:numPr></w:pPr><w:r><w:t>%s</w:t></w:r></w:p>`, index, id, text)
}

func simpleNumbering(levels string) string {
	return `<w:abstractNum w:abstractNumId="0">` + levels + `</w:abstractNum><w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>`
}

func labels(t testing.TB, got word.Resolution) []string {
	t.Helper()
	var labels []string
	for _, p := range got.Paragraphs {
		if p.Numbering == nil {
			labels = append(labels, "(none)")
			continue
		}
		if !p.Numbering.Resolved {
			t.Fatalf("unresolved label at %s: %+v", p.Path, got.Findings)
		}
		labels = append(labels, p.Numbering.Text)
	}
	return labels
}

func TestResolutionCountsNestedListsOverridesAndIndependentInstances(t *testing.T) {
	numbers := simpleNumbering(level(0, 1, "decimal", "%1.", "")+level(1, 1, "lowerLetter", "%1.%2)", `<w:suff w:val="space"/>`)) +
		`<w:num w:numId="2"><w:abstractNumId w:val="0"/><w:lvlOverride w:ilvl="0"><w:startOverride w:val="7"/></w:lvlOverride></w:num>`
	body := numbered(1, 0, "One") + numbered(1, 1, "a") + `<w:p><w:r><w:t>ordinary interruption</w:t></w:r></w:p>` + numbered(1, 1, "b") + numbered(2, 0, "Seven") + numbered(1, 0, "Two") + numbered(1, 1, "a") + numbered(2, 0, "Eight")
	got := resolved(t, resolutionEntries(body, "", numbers))
	want := []string{"1.", "1.a)", "(none)", "1.b)", "7.", "2.", "2.a)", "8."}
	if actual := labels(t, got); !reflect.DeepEqual(actual, want) {
		t.Fatalf("got %v want %v", actual, want)
	}
	if got.Paragraphs[1].Numbering.Suffix != "space" || len(got.Paragraphs[4].Numbering.Sources) != 4 {
		t.Fatalf("lost suffix/override provenance: %+v", got)
	}
}

func TestResolutionStyleLinkedLevelAndExplicitNoNumbering(t *testing.T) {
	styles := `<w:style w:type="paragraph" w:styleId="Question" w:default="1"><w:pPr><w:numPr><w:ilvl w:val="8"/><w:numId w:val="1"/></w:numPr></w:pPr></w:style>`
	numbers := simpleNumbering(level(0, 1, "upperRoman", "%1.", "") + level(1, 1, "upperLetter", "%2)", `<w:pStyle w:val="Question"/><w:rPr><w:vanish/></w:rPr>`))
	body := `<w:p><w:r><w:t>Question</w:t></w:r></w:p><w:p><w:pPr><w:numPr><w:numId w:val="0"/></w:numPr></w:pPr><w:r><w:t>Plain</w:t></w:r></w:p><w:p><w:r><w:t>Next</w:t></w:r></w:p>`
	got := resolved(t, resolutionEntries(body, styles, numbers))
	if actual := labels(t, got); !reflect.DeepEqual(actual, []string{"A)", "(none)", "B)"}) {
		t.Fatal(actual)
	}
	if len(got.Paragraphs[0].Runs[0].Marks) != 0 {
		t.Fatal("number glyph formatting leaked into body")
	}
}

func TestResolutionRestartingStyledListPreservesAssociatedLevel(t *testing.T) {
	styles := `<w:style w:type="paragraph" w:styleId="Question" w:default="1"><w:pPr><w:numPr><w:numId w:val="1"/></w:numPr></w:pPr></w:style>`
	numbers := simpleNumbering(level(0, 1, "decimal", "%1.", "")+level(1, 1, "upperLetter", "%2)", `<w:pStyle w:val="Question"/>`)) +
		`<w:num w:numId="2"><w:abstractNumId w:val="0"/><w:lvlOverride w:ilvl="1"><w:startOverride w:val="4"/></w:lvlOverride></w:num>`
	body := `<w:p><w:r><w:t>inherited</w:t></w:r></w:p><w:p><w:pPr><w:numPr><w:numId w:val="2"/></w:numPr></w:pPr><w:r><w:t>restart</w:t></w:r></w:p>`
	if actual := labels(t, resolved(t, resolutionEntries(body, styles, numbers))); !reflect.DeepEqual(actual, []string{"A)", "D)"}) {
		t.Fatal(actual)
	}
}

func TestResolutionRestartRulesAndLegalNumbering(t *testing.T) {
	for _, tc := range []struct {
		extra string
		want  []string
	}{
		{`<w:lvlRestart w:val="0"/>`, []string{"I.", "I.a)", "II.", "II.b)"}},
		{`<w:isLgl/>`, []string{"I.", "1.1)", "II.", "2.1)"}},
		{`<w:lvlRestart w:val="9"/>`, []string{"I.", "I.a)", "II.", "II.a)"}},
	} {
		numbers := simpleNumbering(level(0, 1, "upperRoman", "%1.", "") + level(1, 1, "lowerLetter", "%1.%2)%3", tc.extra))
		got := resolved(t, resolutionEntries(numbered(1, 0, "x")+numbered(1, 1, "x")+numbered(1, 0, "x")+numbered(1, 1, "x"), "", numbers))
		if actual := labels(t, got); !reflect.DeepEqual(actual, tc.want) {
			t.Fatalf("%s: %v want %v", tc.extra, actual, tc.want)
		}
	}
}

func TestResolutionUnsupportedNumberingIsNotGuessed(t *testing.T) {
	for _, tc := range []struct {
		name, format, pattern, extra string
		start                        int
	}{
		{"custom", "chineseCounting", "%1.", "", 1},
		{"letter overflow", "upperLetter", "%1.", "", 27},
		{"roman overflow", "upperRoman", "%1.", "", 4000},
		{"symbol font", "bullet", "\uf0b7", "", 1},
		{"picture", "bullet", "•", `<w:lvlPicBulletId w:val="1"/>`, 1},
		{"missing template", "decimal", "", "", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			definition := level(0, tc.start, tc.format, tc.pattern, tc.extra)
			if tc.name == "missing template" {
				definition = strings.Replace(definition, `<w:lvlText w:val=""/>`, "", 1)
			}
			got := resolved(t, resolutionEntries(numbered(1, 0, "Question"), "", simpleNumbering(definition)))
			if n := got.Paragraphs[0].Numbering; n == nil || n.Resolved || n.Text != "" || len(got.Findings) == 0 {
				t.Fatalf("guessed %s: %+v", tc.name, got)
			}
		})
	}
}

func TestResolutionRevisionsDoNotShiftTrustedLabels(t *testing.T) {
	numbers := simpleNumbering(level(0, 1, "decimal", "%1.", ""))
	body := numbered(1, 0, "Original") + `<w:del>` + numbered(1, 0, "Deleted") + `</w:del>` + numbered(1, 0, "Next")
	got := resolved(t, resolutionEntries(body, "", numbers))
	for _, p := range got.Paragraphs {
		if p.Numbering == nil || p.Numbering.Resolved {
			t.Fatalf("revision view guessed: %+v", got)
		}
	}
}

func TestResolutionStrictNamespacesOrderingAndCancellation(t *testing.T) {
	entries := resolutionEntries(numbered(1, 0, "Question"), "", simpleNumbering(level(0, 1, "decimal", "%1.", "")))
	transitional := resolved(t, entries)
	for i := range entries {
		entries[i].text = strings.ReplaceAll(entries[i].text, wNS, "http://purl.oclc.org/ooxml/wordprocessingml/main")
	}
	if actual := labels(t, resolved(t, entries)); !reflect.DeepEqual(actual, labels(t, transitional)) {
		t.Fatal(actual)
	}
	source := inspect(t, entries)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := word.Resolve(ctx, source); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	a := resolved(t, entries)
	slices.Reverse(entries)
	if b := resolved(t, entries); !reflect.DeepEqual(a, b) {
		t.Fatal("ZIP entry order changed derived evidence")
	}
}

func TestResolutionDirectFormattingClearsAmbiguousToggle(t *testing.T) {
	styles := `<w:style w:type="paragraph" w:styleId="Base" w:default="1"><w:rPr><w:b/></w:rPr></w:style><w:style w:type="character" w:styleId="Strong"><w:rPr><w:b/></w:rPr></w:style>`
	body := `<w:p><w:r><w:rPr><w:rStyle w:val="Strong"/><w:b w:val="0"/></w:rPr><w:t>plain</w:t></w:r></w:p>`
	got := resolved(t, resolutionEntries(body, styles, ""))
	m := mark(t, got.Paragraphs[0].Runs[0], "b")
	if !m.Resolved || m.Value != "off" || len(m.Sources) != 3 {
		t.Fatalf("direct override failed: %+v", m)
	}
}

func TestResolutionDoesNotGuessConditionalOrScriptMarks(t *testing.T) {
	for _, tc := range []struct{ name, body, styles string }{
		{"table", `<w:tbl><w:tr><w:tc><w:p><w:r><w:rPr><w:u w:val="single"/></w:rPr><w:t>cell</w:t></w:r></w:p></w:tc></w:tr></w:tbl>`, ""},
		{"inherited complex script", `<w:p><w:r><w:t>Text</w:t></w:r></w:p>`, `<w:style w:type="paragraph" w:styleId="a" w:default="1"><w:rPr><w:cs/><w:b/></w:rPr></w:style>`},
		{"non Latin", `<w:p><w:r><w:rPr><w:b/></w:rPr><w:t>العربية</w:t></w:r></w:p>`, ""},
		{"duplicate direct property", `<w:p><w:r><w:rPr><w:u w:val="single"/><w:u w:val="none"/></w:rPr><w:t>Text</w:t></w:r></w:p>`, ""},
		{"duplicate paragraph style", `<w:p><w:pPr><w:pStyle w:val="a"/><w:pStyle w:val="b"/></w:pPr><w:r><w:t>Text</w:t></w:r></w:p>`, `<w:style w:type="paragraph" w:styleId="a"><w:rPr><w:u w:val="single"/></w:rPr></w:style><w:style w:type="paragraph" w:styleId="b"/>`},
		{"invalid underline", `<w:p><w:r><w:rPr><w:u w:val="invented"/></w:rPr><w:t>Text</w:t></w:r></w:p>`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := resolved(t, resolutionEntries(tc.body, tc.styles, ""))
			run := got.Paragraphs[0].Runs[0]
			if run.Complete || len(got.Findings) == 0 {
				t.Fatalf("unsupported styling accepted: %+v", got)
			}
			for _, m := range run.Marks {
				if m.Resolved || m.Value != "" {
					t.Fatalf("unsupported mark guessed: %+v", m)
				}
			}
		})
	}
}

func TestResolutionRejectsMalformedListReferencesAndDefinitions(t *testing.T) {
	base := simpleNumbering(level(0, 1, "decimal", "%1.", ""))
	for _, tc := range []struct{ name, body, numbers string }{
		{"missing instance", numbered(7, 0, "text"), base},
		{"invalid level", numbered(1, 9, "text"), base},
		{"negative level", numbered(1, -1, "text"), base},
		{"missing level", numbered(1, 3, "text"), base},
		{"duplicate normalized instance", numbered(1, 0, "text"), base + `<w:num w:numId="01"><w:abstractNumId w:val="0"/></w:num>`},
		{"duplicate override", numbered(1, 0, "text"), strings.Replace(base, `</w:num>`, `<w:lvlOverride w:ilvl="0"><w:startOverride w:val="3"/></w:lvlOverride><w:lvlOverride w:ilvl="0"><w:startOverride w:val="7"/></w:lvlOverride></w:num>`, 1)},
		{"invalid start", numbered(1, 0, "text"), strings.Replace(base, `<w:start w:val="1"/>`, `<w:start w:val="-1"/>`, 1)},
		{"custom format", numbered(1, 0, "text"), strings.Replace(base, `w:val="decimal"`, `w:val="decimal" w:format="custom"`, 1)},
		{"style link", numbered(1, 0, "text"), strings.Replace(base, `</w:abstractNum>`, `<w:numStyleLink w:val="ListStyle"/></w:abstractNum>`, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := resolved(t, resolutionEntries(tc.body, "", tc.numbers))
			if n := got.Paragraphs[0].Numbering; n == nil || n.Resolved || n.Text != "" || len(got.Findings) == 0 {
				t.Fatalf("invalid list accepted: %+v", got)
			}
		})
	}
}

func TestResolutionDefaultStartAndFormatAndLevelOverride(t *testing.T) {
	base := simpleNumbering(`<w:lvl w:ilvl="0"><w:lvlText w:val="%1."/></w:lvl>`)
	got := resolved(t, resolutionEntries(numbered(1, 0, "zero")+numbered(1, 0, "one"), "", base))
	if actual := labels(t, got); !reflect.DeepEqual(actual, []string{"0.", "1."}) {
		t.Fatal(actual)
	}
	replacement := `<w:lvlOverride w:ilvl="0"><w:startOverride w:val="7"/>` + level(0, 3, "upperLetter", "%1)", "") + `</w:lvlOverride>`
	base = strings.Replace(base, `</w:num>`, replacement+`</w:num>`, 1)
	got = resolved(t, resolutionEntries(numbered(1, 0, "seven"), "", base))
	if actual := labels(t, got); !reflect.DeepEqual(actual, []string{"G)"}) {
		t.Fatal(actual)
	}
}

func TestResolutionTextboxesAndAncillaryPartsDoNotAdvanceBodyCounter(t *testing.T) {
	entries := resolutionEntries(numbered(1, 0, "one")+`<w:p><w:r><w:pict><w:txbxContent>`+numbered(1, 0, "box")+`</w:txbxContent></w:pict></w:r></w:p>`+numbered(1, 0, "two"), "", simpleNumbering(level(0, 1, "decimal", "%1.", "")))
	entries[0].text = strings.Replace(entries[0].text, `</Types>`, `<Override PartName="/word/aaa-header.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/></Types>`, 1)
	entries = append(entries, entry{"word/aaa-header.xml", `<w:hdr xmlns:w="` + wNS + `">` + numbered(7, 8, "header") + `</w:hdr>`})
	got := resolved(t, entries)
	var bodyLabels []string
	for _, p := range got.Paragraphs {
		if p.Numbering != nil && p.Numbering.Resolved {
			bodyLabels = append(bodyLabels, p.Numbering.Text)
		}
	}
	if !reflect.DeepEqual(bodyLabels, []string{"1.", "2."}) {
		t.Fatal(bodyLabels)
	}
}

func TestResolutionUsesRelationshipsInsteadOfConventionalFilenames(t *testing.T) {
	entries := resolutionEntries(numbered(1, 0, "text"), "", simpleNumbering(level(0, 4, "decimal", "%1.", "")))
	for i := range entries {
		entries[i].name = strings.ReplaceAll(entries[i].name, "numbering.xml", "lists.xml")
		entries[i].text = strings.ReplaceAll(entries[i].text, "numbering.xml", "lists.xml")
	}
	if got := labels(t, resolved(t, entries)); !reflect.DeepEqual(got, []string{"4."}) {
		t.Fatal(got)
	}
	for i := range entries {
		if entries[i].name == "word/_rels/document.xml.rels" {
			entries[i].text = `<Relationships xmlns="` + rNS + `"/>`
		}
	}
	got := resolved(t, entries)
	if got.Paragraphs[0].Numbering.Resolved {
		t.Fatal("unlinked numbering part was trusted")
	}
}

func TestResolutionBoundsStyleExpansion(t *testing.T) {
	var styles strings.Builder
	for i := 0; i < 60; i++ {
		parent := ""
		if i > 0 {
			parent = fmt.Sprintf(`<w:basedOn w:val="s%d"/>`, i-1)
		}
		fmt.Fprintf(&styles, `<w:style w:type="paragraph" w:styleId="s%d">%s<w:rPr><w:b/><w:i/><w:u w:val="single"/><w:strike/><w:vanish/></w:rPr></w:style>`, i, parent)
	}
	body := `<w:p><w:pPr><w:pStyle w:val="s59"/></w:pPr>` + strings.Repeat(`<w:r><w:t>x</w:t></w:r>`, 3000) + `</w:p>`
	_, err := word.Resolve(context.Background(), inspect(t, resolutionEntries(body, styles.String(), "")))
	if !errors.Is(err, word.ErrLimit) {
		t.Fatalf("expected bounded failure, got %v", err)
	}
}

func TestResolutionBoundsRepeatedLargeStyleIdentifiers(t *testing.T) {
	id := strings.Repeat("s", 100000)
	styles := `<w:style w:type="paragraph" w:styleId="` + id + `" w:default="1"><w:rPr><w:u w:val="single"/></w:rPr></w:style>`
	body := strings.Repeat(`<w:p><w:r><w:t>small paragraph</w:t></w:r></w:p>`, 40)
	_, err := word.Resolve(context.Background(), inspect(t, resolutionEntries(body, styles, "")))
	if !errors.Is(err, word.ErrLimit) {
		t.Fatalf("expected bounded work, got %v", err)
	}
}

func BenchmarkResolveFiftyNumberedParagraphs(b *testing.B) {
	var body strings.Builder
	for i := 0; i < 50; i++ {
		body.WriteString(numbered(1, 0, "Synthetic question"))
	}
	source := inspect(b, resolutionEntries(body.String(), "", simpleNumbering(level(0, 1, "decimal", "%1.", ""))))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := word.Resolve(context.Background(), source); err != nil {
			b.Fatal(err)
		}
	}
}
