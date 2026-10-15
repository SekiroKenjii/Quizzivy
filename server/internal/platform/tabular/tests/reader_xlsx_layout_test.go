package tabular_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/platform/tabular"
)

func replaceBody(entries []entry, name, old, replacement string) []entry {
	out := make([]entry, len(entries))
	copy(out, entries)
	for i, e := range out {
		if e.name == name {
			out[i].body = strings.Replace(e.body, old, replacement, 1)
		}
	}
	return out
}

func withSheetAt(t *testing.T, target, part string) []byte {
	t.Helper()
	entries := replaceBody(workbookParts(nil), "xl/_rels/workbook.xml.rels", `Target="worksheets/sheet1.xml"`, `Target="`+target+`"`)
	return archive(t, append(entries, entry{name: part, body: sheetEntry(rosterRows(2)).body})...)
}

func TestASheetOutsideTheStandardFolderIsALayoutErrorNotAMissingHeader(t *testing.T) {
	for name, tc := range map[string]struct{ target, part string }{
		"a folder of its own": {"sheets/s1.xml", "xl/sheets/s1.xml"},
		"beside the workbook": {"worksheet1.xml", "xl/worksheet1.xml"},
		"the package root":    {"/sheet1.xml", "sheet1.xml"},
		"no extension":        {"worksheets_sheet1", "xl/worksheets_sheet1"},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := read(t, withSheetAt(t, tc.target, tc.part), tabular.FormatXLSX, tabular.DefaultLimits())

			if !errors.Is(err, tabular.ErrLayout) || !errors.Is(err, tabular.ErrFormat) {
				t.Errorf("err = %v, want ErrLayout, which wraps ErrFormat", err)
			}
			if errors.Is(err, tabular.ErrEmpty) {
				t.Errorf("err = %v, must not tell the teacher the header is missing", err)
			}
		})
	}

	_, rows, err := read(t, withSheetAt(t, "/xl/worksheets/sheet1.xml", "xl/worksheets/sheet1.xml"), tabular.FormatXLSX, tabular.DefaultLimits())
	if err != nil || len(rows) != 2 {
		t.Errorf("an absolute target under xl/worksheets/: rows = %d, err = %v, want it read", len(rows), err)
	}
}

func TestAnArchiveWithNoWorksheetAtAllIsALayoutError(t *testing.T) {
	_, _, err := read(t, archive(t, workbookParts(nil)...), tabular.FormatXLSX, tabular.DefaultLimits())

	if !errors.Is(err, tabular.ErrLayout) {
		t.Errorf("err = %v, want ErrLayout", err)
	}
}

func hugeWorkbookAt(t *testing.T, part string, chunks int) rawPart {
	t.Helper()
	return deflatedPart(t, part, func(emit func(string)) {
		emit(`<workbook xmlns="` + spreadsheetNS + `">`)
		nest := strings.Repeat("<a>", 1<<20)
		for range chunks {
			emit(nest)
		}
	})
}

func TestTheWorkbookMustBeWhereExcelizeWillDecodeOnlyWhatWasScanned(t *testing.T) {
	const document = relationsNS + "/officeDocument"
	rels := func(relationships ...string) entry {
		return entry{"_rels/.rels", `<?xml version="1.0"?><Relationships xmlns="` + packageRelsNS + `">` + strings.Join(relationships, "") + `</Relationships>`}
	}
	rel := func(id, kind, target string) string {
		return `<Relationship Id="` + id + `" Type="` + kind + `" Target="` + target + `"/>`
	}
	withRels := func(root entry) []entry {
		entries := workbookParts(nil)
		for i, e := range entries {
			if e.name == root.name {
				entries[i] = root
			}
		}
		return append(entries, sheetEntry(rosterRows(2)))
	}

	for name, root := range map[string]entry{
		"a part with no limit of its own":     rels(rel("rId1", document, "blob.bin")),
		"an xml part":                         rels(rel("rId1", document, "other.xml")),
		"a strict type":                       rels(rel("rId1", "http://purl.oclc.org/ooxml/officeDocument/relationships/officeDocument", "blob.bin")),
		"a type in capitals":                  rels(rel("rId1", strings.ToUpper(document), "blob.bin")),
		"after a good one":                    rels(rel("rId1", document, "xl/workbook.xml"), rel("rId2", document, "blob.bin")),
		"before a good one":                   rels(rel("rId1", document, "blob.bin"), rel("rId2", document, "xl/workbook.xml")),
		"another case":                        rels(rel("rId1", document, "XL/Workbook.xml")),
		"a path that cleans to the right one": rels(rel("rId1", document, "xl/../xl/workbook.xml")),
		"a leading dot":                       rels(rel("rId1", document, "./xl/workbook.xml")),
		"a namespaced attribute":              {"_rels/.rels", `<Relationships xmlns="` + packageRelsNS + `" xmlns:x="urn:x"><Relationship Id="rId1" x:Type="` + document + `" x:Target="blob.bin"/></Relationships>`},
	} {
		t.Run(name, func(t *testing.T) {
			data := archiveRaw(t, withRels(root), hugeWorkbookAt(t, "blob.bin", 6))
			var err error
			start := time.Now()

			allocated := allocatedDuring(func() { _, _, err = read(t, data, tabular.FormatXLSX, tabular.DefaultLimits()) })

			if !errors.Is(err, tabular.ErrLayout) {
				t.Errorf("err = %v, want ErrLayout", err)
			}
			if allocated > 8<<20 || time.Since(start) > 2*time.Second {
				t.Errorf("refusing it took %v and allocated %d bytes, want the six million nested elements left undecoded", time.Since(start), allocated)
			}
		})
	}

	for name, root := range map[string]entry{
		"the usual target":           rels(rel("rId1", document, "xl/workbook.xml")),
		"an absolute target":         rels(rel("rId1", document, "/xl/workbook.xml")),
		"a thumbnail beside it":      rels(rel("rId1", document, "xl/workbook.xml"), rel("rId2", "http://schemas.openxmlformats.org/package/2006/relationships/metadata/thumbnail", "docProps/thumbnail.jpeg")),
		"a core properties relation": rels(rel("rId1", "http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties", "docProps/core.xml"), rel("rId2", document, "xl/workbook.xml")),
	} {
		t.Run(name, func(t *testing.T) {
			_, rows, err := read(t, archive(t, withRels(root)...), tabular.FormatXLSX, tabular.DefaultLimits())

			if err != nil || len(rows) != 2 {
				t.Errorf("rows = %d, err = %v, want the 2 rows read", len(rows), err)
			}
		})
	}
}

func TestEveryPartExcelizeCouldReadAsTheSheetIsHeldToTheSheetRules(t *testing.T) {
	styles := func(inner string) entry {
		return entry{"xl/styles.xml", `<styleSheet xmlns="` + spreadsheetNS + `"><sheetData>` + inner + `</sheetData></styleSheet>`}
	}
	calcChain := func(inner string) entry {
		return entry{"xl/calcChain.xml", `<calcChain xmlns="` + spreadsheetNS + `"><sheetData>` + inner + `</sheetData></calcChain>`}
	}
	named := row(1, text("A1", "name"))
	for name, tc := range map[string]struct {
		target string
		part   entry
		want   error
	}{
		"a styles part with a row past the sheet":    {"styles.xml", styles(named + `<row r="2147483647"/>`), tabular.ErrFormat},
		"a calc chain with a row past the sheet":     {"calcChain.xml", calcChain(named + `<row r="2147483647"/>`), tabular.ErrFormat},
		"a styles part with a row of too many cells": {"styles.xml", styles(`<row r="1">` + strings.Repeat(`<c r="A1"/>`, 90000) + `</row>`), tabular.ErrTooManyColumns},
		"a calc chain with a row of too many cells":  {"calcChain.xml", calcChain(`<row r="1">` + strings.Repeat(`<c r="A1"/>`, 90000) + `</row>`), tabular.ErrTooManyColumns},
	} {
		t.Run(name, func(t *testing.T) {
			entries := replaceBody(workbookParts(nil), "xl/_rels/workbook.xml.rels", `Target="worksheets/sheet1.xml"`, `Target="`+tc.target+`"`)
			for i, e := range entries {
				if e.name == tc.part.name {
					entries = append(entries[:i:i], entries[i+1:]...)
					break
				}
			}
			data := archive(t, append(entries, tc.part, sheetEntry(rosterRows(1)))...)
			var err error
			start := time.Now()

			allocated := allocatedDuring(func() { _, _, err = read(t, data, tabular.FormatXLSX, tabular.DefaultLimits()) })

			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
			if time.Since(start) > 2*time.Second || allocated > 16<<20 {
				t.Errorf("took %v and allocated %d bytes, want the rows never walked", time.Since(start), allocated)
			}
		})
	}
}

func TestACalculationChainOfManyCellsIsNotMistakenForARow(t *testing.T) {
	chain := entry{"xl/calcChain.xml", `<calcChain xmlns="` + spreadsheetNS + `">` + strings.Repeat(`<c r="A1" i="1"/>`, 50000) + `</calcChain>`}

	_, rows, err := read(t, workbook(t, rosterRows(2), nil, chain), tabular.FormatXLSX, tabular.DefaultLimits())

	if err != nil || len(rows) != 2 {
		t.Errorf("rows = %d, err = %v, want the 2 rows read beside a 50,000-cell calculation chain", len(rows), err)
	}
}

func TestEachSizeSentinelHasItsOwnMessage(t *testing.T) {
	seen := map[string]error{}
	for _, sentinel := range sizeSentinels {
		message := sentinel.Error()
		if message == "" || seen[message] != nil {
			t.Errorf("%q is empty or shared with %v", message, seen[message])
		}
		seen[message] = sentinel
	}
	if !strings.Contains(tabular.ErrSheetTooLarge.Error(), "8 MiB") {
		t.Errorf("%q, want it to name the 8 MiB limit", tabular.ErrSheetTooLarge)
	}
}
