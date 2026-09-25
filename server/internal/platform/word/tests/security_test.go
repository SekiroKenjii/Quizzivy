package word_test

import (
	"bytes"
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"quizzivy/internal/platform/word"
)

func TestMalformedAndActivePackagesAreRejected(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func([]entry) []entry
		want   error
	}{
		{"traversal", func(es []entry) []entry { return append(es, entry{"../escape.xml", "bad"}) }, word.ErrInvalidPackage},
		{"backslash", func(es []entry) []entry { return append(es, entry{`word\escape.xml`, "bad"}) }, word.ErrInvalidPackage},
		{"duplicate", func(es []entry) []entry { return append(es, es[2]) }, word.ErrInvalidPackage},
		{"macros", func(es []entry) []entry { return append(es, entry{"word/vbaProject.bin", "payload"}) }, word.ErrActiveContent},
		{"ole", func(es []entry) []entry { return append(es, entry{"word/embeddings/object.bin", "payload"}) }, word.ErrActiveContent},
		{"DTD", func(es []entry) []entry {
			es[2].text = `<!DOCTYPE document [<!ENTITY x SYSTEM "file:///etc/passwd">]>` + document(`<w:p><w:r><w:t>&x;</w:t></w:r></w:p>`)
			return es
		}, word.ErrInvalidPackage},
		{"wrong namespace", func(es []entry) []entry { es[2].text = strings.ReplaceAll(es[2].text, wNS, "urn:fake"); return es }, word.ErrInvalidPackage},
		{"multiple roots", func(es []entry) []entry { es[2].text += `<another/>`; return es }, word.ErrInvalidPackage},
		{"truncated XML", func(es []entry) []entry { es[2].text = `<w:document xmlns:w="` + wNS + `">`; return es }, word.ErrInvalidPackage},
		{"duplicate XML attribute", func(es []entry) []entry {
			es[2].text = document(`<w:p><w:r><w:rPr><w:u w:val="single" w:val="none"/></w:rPr></w:r></w:p>`)
			return es
		}, word.ErrInvalidPackage},
		{"missing root relations", func(es []entry) []entry { return []entry{es[0], es[2]} }, word.ErrInvalidPackage},
		{"macro content type", func(es []entry) []entry {
			es[0].text = strings.ReplaceAll(es[0].text, mainType, "application/vnd.ms-word.document.macroEnabled.main+xml")
			return es
		}, word.ErrActiveContent},
		{"outside relation", func(es []entry) []entry {
			es[1].text = strings.ReplaceAll(es[1].text, "word/document.xml", "../../outside.xml")
			return es
		}, word.ErrInvalidPackage},
		{"encoded outside relation", func(es []entry) []entry {
			es[1].text = strings.ReplaceAll(es[1].text, "word/document.xml", "%2e%2e/outside.xml")
			return es
		}, word.ErrInvalidPackage},
		{"unmarked network target", func(es []entry) []entry {
			es[1].text = strings.ReplaceAll(es[1].text, "word/document.xml", "https://example.com/exam.xml")
			return es
		}, word.ErrInvalidPackage},
		{"external main", func(es []entry) []entry {
			es[1].text = strings.ReplaceAll(es[1].text, `Target="word/document.xml"`, `Target="https://example.com/exam.xml" TargetMode="External"`)
			return es
		}, word.ErrInvalidPackage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := pack(t, tc.mutate(baseEntries(`<w:p><w:r><w:t>Content</w:t></w:r></w:p>`)))
			_, err := word.Inspect(context.Background(), bytes.NewReader(data), int64(len(data)), word.DefaultLimits())
			if !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
		})
	}
}

func TestXMLBudgetIsSharedByAllParts(t *testing.T) {
	entries := baseEntries(`<w:p><w:r><w:t>Content</w:t></w:r></w:p>`)
	data := pack(t, entries)
	limits := word.DefaultLimits()
	limits.XMLTotalBytes = int64(len(entries[0].text) + len(entries[1].text))
	_, err := word.Inspect(context.Background(), bytes.NewReader(data), int64(len(data)), limits)
	if !errors.Is(err, word.ErrLimit) {
		t.Fatalf("individually small XML parts must share one budget, got %v", err)
	}
}

func TestDeepLongNamespaceCannotExhaustLocatorMemory(t *testing.T) {
	uri := "urn:" + strings.Repeat("n", 2048)
	body := `<x:wrapper xmlns:x="` + uri + `">` + strings.Repeat(`<x:child>`, 30) + `<w:p/>` + strings.Repeat(`</x:child>`, 30) + `</x:wrapper>`
	data := pack(t, baseEntries(body))
	limits := word.DefaultLimits()
	limits.LocatorBytes = 64 << 10
	_, err := word.Inspect(context.Background(), bytes.NewReader(data), int64(len(data)), limits)
	if !errors.Is(err, word.ErrLimit) {
		t.Fatalf("amplified locators must hit a budget, got %v", err)
	}
}

func TestEveryResourceBudgetIsEnforced(t *testing.T) {
	data := pack(t, baseEntries(`<w:p><w:r><w:t>Bounded content</w:t></w:r></w:p>`))
	for _, tc := range []struct {
		name   string
		change func(*word.Limits)
	}{
		{"compressed bytes", func(l *word.Limits) { l.CompressedBytes = 1 }},
		{"expanded bytes", func(l *word.Limits) { l.ExpandedBytes = 1 }},
		{"entries", func(l *word.Limits) { l.Entries = 1 }},
		{"XML bytes", func(l *word.Limits) { l.XMLBytes = 4 }},
		{"total XML bytes", func(l *word.Limits) { l.XMLTotalBytes = 4 }},
		{"locator bytes", func(l *word.Limits) { l.LocatorBytes = 4 }},
		{"overflowing XML limit", func(l *word.Limits) { l.XMLBytes = math.MaxInt64 }},
		{"XML nodes across parts", func(l *word.Limits) { l.XMLNodes = 4 }},
		{"XML depth", func(l *word.Limits) { l.XMLDepth = 2 }},
		{"invalid limit", func(l *word.Limits) { l.XMLDepth = 0 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			limits := word.DefaultLimits()
			tc.change(&limits)
			_, err := word.Inspect(context.Background(), bytes.NewReader(data), int64(len(data)), limits)
			if !errors.Is(err, word.ErrLimit) {
				t.Fatalf("want a resource limit failure, got %v", err)
			}
		})
	}
}

func TestNonZipAndLegacyInputsNeverUseALossyFallback(t *testing.T) {
	for _, tc := range []struct {
		data []byte
		want error
	}{
		{[]byte("not a Word file"), word.ErrInvalidPackage},
		{[]byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}, word.ErrLegacyOrLocked},
	} {
		_, err := word.Inspect(context.Background(), bytes.NewReader(tc.data), int64(len(tc.data)), word.DefaultLimits())
		if !errors.Is(err, tc.want) {
			t.Fatalf("want %v, got %v", tc.want, err)
		}
	}
}

func FuzzInspectionNeverPanicsOrReadsOutsideThePackage(f *testing.F) {
	f.Add(pack(f, baseEntries(`<w:p><w:r><w:t>Seed</w:t></w:r></w:p>`)))
	f.Add(pack(f, resolutionEntries(numbered(1, 0, "Seed"), "", simpleNumbering(level(0, 1, "decimal", "%1.", "")))))
	f.Add([]byte("not a ZIP"))
	f.Fuzz(func(_ *testing.T, data []byte) {
		limits := word.DefaultLimits()
		limits.CompressedBytes, limits.ExpandedBytes, limits.XMLBytes = 1<<16, 1<<18, 1<<16
		limits.XMLNodes, limits.XMLDepth, limits.Entries = 2000, 24, 32
		source, err := word.Inspect(context.Background(), bytes.NewReader(data), int64(len(data)), limits)
		if err == nil {
			_, _ = word.Resolve(context.Background(), source)
		}
	})
}

func TestInertCustomXMLDoesNotRejectAnOrdinaryExam(t *testing.T) {
	entries := append(baseEntries(`<w:p><w:r><w:t>Question 1</w:t></w:r></w:p>`),
		entry{"customXml/item1.xml", `<?xml version="1.0"?><?mso-contentType ?><FormTemplates xmlns="http://schemas.microsoft.com/sharepoint/v3/contenttype/forms"/>`},
		entry{"customXml/item2.xml", `<?xml version="1.0" encoding="UTF-16"?><b:Sources xmlns:b="urn:bibliography"/>`},
	)
	data := pack(t, entries)
	got, err := word.Inspect(context.Background(), bytes.NewReader(data), int64(len(data)), word.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if !finding(got, "UNSUPPORTED_SOURCE_PART") {
		t.Fatalf("unreadable custom XML not recorded: %+v", got.Findings)
	}
}

func TestAMisplacedXMLDeclarationIsStillMalformed(t *testing.T) {
	entries := baseEntries(`<w:p><w:r><w:t>Text</w:t></w:r></w:p><?xml version="1.0"?>`)
	data := pack(t, entries)
	if _, err := word.Inspect(context.Background(), bytes.NewReader(data), int64(len(data)), word.DefaultLimits()); !errors.Is(err, word.ErrInvalidPackage) {
		t.Fatalf("err %v", err)
	}
}

func TestAnArchiveDeclaringTooManyEntriesIsRejectedBeforeReadingThem(t *testing.T) {
	data := pack(t, baseEntries(`<w:p><w:r><w:t>Text</w:t></w:r></w:p>`))
	eocd := bytes.LastIndex(data, []byte{0x50, 0x4b, 0x05, 0x06})
	forged := bytes.Clone(data)
	forged[eocd+8], forged[eocd+9], forged[eocd+10], forged[eocd+11] = 0x30, 0x75, 0x30, 0x75
	_, err := word.Inspect(context.Background(), bytes.NewReader(forged), int64(len(forged)), word.DefaultLimits())
	if !errors.Is(err, word.ErrLimit) {
		t.Fatalf("err %v", err)
	}
}
