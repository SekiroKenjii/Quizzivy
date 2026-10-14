package tabular_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/xuri/excelize/v2"

	"quizzivy/internal/platform/tabular"
)

const worksheetSize = 8 << 20

func rosterRows(n int) string {
	var rows strings.Builder
	rows.WriteString(row(1, text("A1", "name"), text("B1", "email")))
	for i := 2; i <= n+1; i++ {
		rows.WriteString(row(i, text(fmt.Sprintf("A%d", i), fmt.Sprintf("Person %d", i)), text(fmt.Sprintf("B%d", i), fmt.Sprintf("p%d@example.com", i))))
	}
	return rows.String()
}

func rosterSheet(n int) []byte {
	return []byte(sheetEntry(rosterRows(n)).body)
}

func nestedWorksheet(t *testing.T, name string, chunks int) rawPart {
	t.Helper()
	return deflatedPart(t, name, func(emit func(string)) {
		emit(`<worksheet xmlns="` + spreadsheetNS + `"><sheetData>` + row(1, text("A1", "name")))
		chunk := strings.Repeat("<a>", 1<<20)
		for range chunks {
			emit(chunk)
		}
	})
}

func TestAWorksheetOverTheSizeLimitIsRefusedBeforeItIsParsed(t *testing.T) {
	for name, part := range map[string]rawPart{
		"ten million nested elements":      nestedWorksheet(t, "xl/worksheets/sheet1.xml", 10),
		"rows past the limit":              deflatedPart(t, "xl/worksheets/sheet1.xml", func(emit func(string)) { emit(string(rosterSheet(100000))) }),
		"a sheet under a name without xml": nestedWorksheet(t, "xl/worksheets/sheet1.bin", 10),
	} {
		t.Run(name, func(t *testing.T) {
			data := archiveRaw(t, workbookParts(nil), part)
			if len(data) > 2<<20 {
				t.Fatalf("the test file is %d bytes, want a small one that expands", len(data))
			}
			var err error

			allocated := allocatedDuring(func() { _, _, err = read(t, data, tabular.FormatXLSX, tabular.DefaultLimits()) })

			wantSizeError(t, err, tabular.ErrSheetTooLarge)
			if allocated > 8<<20 {
				t.Errorf("refusing it allocated %d bytes, want the parsing it would have cost left undone", allocated)
			}
		})
	}
}

func TestNestingBeyondSixtyFourLevelsIsRefusedAtOnce(t *testing.T) {
	deep := func(levels int) []byte {
		body := rosterRows(2) + strings.Repeat("<a>", levels) + strings.Repeat("</a>", levels)
		return workbook(t, body, nil)
	}

	_, rows, err := read(t, deep(62), tabular.FormatXLSX, tabular.DefaultLimits())
	if err != nil || len(rows) != 2 {
		t.Fatalf("a sheet 64 levels deep: rows = %v, err = %v, want it read", rows, err)
	}
	if _, _, err := read(t, deep(63), tabular.FormatXLSX, tabular.DefaultLimits()); !errors.Is(err, tabular.ErrFormat) {
		t.Errorf("a sheet 65 levels deep: err = %v, want ErrFormat", err)
	}

	bomb := nestedWorksheet(t, "xl/worksheets/sheet1.xml", 2)
	data := archiveRaw(t, workbookParts(nil), bomb)
	var bombErr error
	allocated := allocatedDuring(func() { _, _, bombErr = read(t, data, tabular.FormatXLSX, tabular.DefaultLimits()) })
	if !errors.Is(bombErr, tabular.ErrFormat) {
		t.Errorf("six million nested elements: err = %v, want ErrFormat", bombErr)
	}
	if allocated > 8<<20 {
		t.Errorf("refusing them allocated %d bytes, want the nesting left unbuilt", allocated)
	}
}

func TestNestingIsAlsoCappedInThePartsExcelizeDecodesWhole(t *testing.T) {
	const drawingNS = "http://schemas.openxmlformats.org/drawingml/2006/main"
	deep := func(root, ns string) string {
		return "<" + root + ` xmlns="` + ns + `">` + strings.Repeat("<a>", 300000) + "</" + root + ">"
	}
	for name, part := range map[string]entry{
		"styles":         {name: "xl/styles.xml", body: deep("styleSheet", spreadsheetNS)},
		"calc chain":     {name: "xl/calcChain.xml", body: deep("calcChain", spreadsheetNS)},
		"shared strings": {name: "xl/sharedStrings.xml", body: deep("sst", spreadsheetNS)},
		"theme":          {name: "xl/theme/theme1.xml", body: deep("theme", drawingNS)},
	} {
		t.Run(name, func(t *testing.T) {
			data := archive(t, append(withoutPart(workbookParts(nil), part.name), sheetEntry(rosterRows(1)), part)...)
			var err error

			allocated := allocatedDuring(func() { _, _, err = read(t, data, tabular.FormatXLSX, tabular.DefaultLimits()) })

			if !errors.Is(err, tabular.ErrFormat) {
				t.Fatalf("err = %v, want ErrFormat", err)
			}
			if allocated > 8<<20 {
				t.Errorf("allocated %d bytes", allocated)
			}
		})
	}
}

func TestNestingIsAlsoCappedInTheWorkbookAndItsRelationships(t *testing.T) {
	for name, root := range map[string]string{
		"xl/workbook.xml":            "workbook",
		"xl/_rels/workbook.xml.rels": "Relationships",
		"_rels/.rels":                "Relationships",
	} {
		t.Run(name, func(t *testing.T) {
			entries := workbookParts(nil)
			for i, e := range entries {
				if e.name == name {
					entries[i].body = strings.Replace(e.body, "</"+root+">", strings.Repeat("<a>", 300000)+"</"+root+">", 1)
				}
			}
			var err error

			allocated := allocatedDuring(func() {
				_, _, err = read(t, archive(t, append(entries, sheetEntry(rosterRows(1)))...), tabular.FormatXLSX, tabular.DefaultLimits())
			})

			if !errors.Is(err, tabular.ErrFormat) {
				t.Fatalf("err = %v, want ErrFormat", err)
			}
			if allocated > 8<<20 {
				t.Errorf("allocated %d bytes", allocated)
			}
		})
	}
}

func TestDirectoryEntriesInTheArchiveAreHarmless(t *testing.T) {
	entries := append([]entry{{name: "xl/"}, {name: "xl/worksheets/"}, {name: "xl/theme/"}}, workbookParts(nil)...)
	data := archive(t, append(entries, sheetEntry(rosterRows(2)))...)

	_, rows, err := read(t, data, tabular.FormatXLSX, tabular.DefaultLimits())

	if err != nil || len(rows) != 2 {
		t.Errorf("rows = %d, err = %v, want the 2 rows read", len(rows), err)
	}
}

func withoutPart(entries []entry, name string) []entry {
	var kept []entry
	for _, e := range entries {
		if e.name != name {
			kept = append(kept, e)
		}
	}
	return kept
}

func TestMoreCellsInARowThanASheetHoldsAreRefused(t *testing.T) {
	cells := func(n int) string {
		var b strings.Builder
		b.WriteString(text("A2", "v"))
		for c := 2; c <= n; c++ {
			name, err := excelize.ColumnNumberToName(min(c, 16384))
			if err != nil {
				t.Fatal(err)
			}
			fmt.Fprintf(&b, `<c r="%s2" s="1"/>`, name)
		}
		return row(1, text("A1", "name")) + row(2, b.String())
	}

	_, rows, err := read(t, workbook(t, cells(16384), nil), tabular.FormatXLSX, tabular.DefaultLimits())
	if err != nil || len(rows) != 1 {
		t.Fatalf("a row of 16384 cells: rows = %v, err = %v, want it read", rows, err)
	}
	if _, _, err := read(t, workbook(t, cells(16385), nil), tabular.FormatXLSX, tabular.DefaultLimits()); !errors.Is(err, tabular.ErrTooManyColumns) {
		t.Errorf("a row of 16385 cells: err = %v, want ErrTooManyColumns", err)
	}

	loose := workbook(t, row(1, text("A1", "name"))+strings.Repeat(`<c r="A2"/>`, 20000), nil)
	if _, _, err := read(t, loose, tabular.FormatXLSX, tabular.DefaultLimits()); !errors.Is(err, tabular.ErrTooManyColumns) {
		t.Errorf("20000 cells outside any row: err = %v, want ErrTooManyColumns", err)
	}
	flood := workbook(t, row(1, text("A1", "name"))+`<row r="2">`+strings.Repeat(`<c r="A2"/>`, 699000)+`</row>`, nil)
	var floodErr error
	allocated := allocatedDuring(func() { _, _, floodErr = read(t, flood, tabular.FormatXLSX, tabular.DefaultLimits()) })
	if !errors.Is(floodErr, tabular.ErrTooManyColumns) {
		t.Errorf("699000 cells in a row: err = %v, want ErrTooManyColumns", floodErr)
	}
	if allocated > 16<<20 {
		t.Errorf("refusing them allocated %d bytes, want the row left unbuilt", allocated)
	}
}

func TestARowNumberOutsideTheSheetIsRefused(t *testing.T) {
	for _, attribute := range []string{`r="1048577"`, `r="2147483647"`, `r="0"`, `r="-3"`, `r="x"`, `xmlns:x="urn:x" x:r="2147483647"`} {
		t.Run(attribute, func(t *testing.T) {
			data := workbook(t, row(1, text("A1", "name"))+`<row `+attribute+`><c r="A5" t="inlineStr"><is><t>v</t></is></c></row>`, nil)

			_, _, err := read(t, data, tabular.FormatXLSX, tabular.DefaultLimits())

			if !errors.Is(err, tabular.ErrFormat) {
				t.Errorf("err = %v, want ErrFormat", err)
			}
		})
	}

	_, rows, err := read(t, workbook(t, row(1, text("A1", "name"))+row(1048576, text("A1048576", "last")), nil), tabular.FormatXLSX, tabular.DefaultLimits())
	if err != nil || len(rows) != 1 || rows[0].Line != 1048576 {
		t.Errorf("the last row of a sheet: rows = %v, err = %v, want it read at line 1048576", rows, err)
	}
}

func TestAStartTagLongerThanTheLimitIsRefusedBeforeItsAttributesAreDecoded(t *testing.T) {
	for name, tag := range map[string]string{
		"1.6 million attributes":  `<c ` + strings.Repeat(`a="" `, 1<<20/5*8/5) + `/>`,
		"namespace declarations":  `<c ` + strings.Repeat(`xmlns:n="u" `, 1<<20/12*4) + `/>`,
		"one huge value":          `<c a="` + strings.Repeat("x", 4<<20) + `"/>`,
		"after a quoted >":        `<c a=">" ` + strings.Repeat(`a="" `, 1<<20/5*8/5) + `/>`,
		"after a single-quoted >": `<c a='>' ` + strings.Repeat(`a="" `, 1<<20/5*8/5) + `/>`,
	} {
		t.Run(name, func(t *testing.T) {
			data := workbook(t, row(1, text("A1", "name"))+`<row r="2">`+tag+`</row>`, nil)
			var err error

			allocated := allocatedDuring(func() { _, _, err = read(t, data, tabular.FormatXLSX, tabular.DefaultLimits()) })

			if !errors.Is(err, tabular.ErrFormat) {
				t.Fatalf("err = %v, want ErrFormat", err)
			}
			if allocated > 16<<20 {
				t.Errorf("refusing it allocated %d bytes, want the attributes left undecoded", allocated)
			}
		})
	}

	long := workbook(t, row(1, text("A1", "name"))+`<row r="2"><c r="A2" t="inlineStr" a="`+strings.Repeat("x", 60<<10)+`"><is><t>v</t></is></c></row>`, nil)
	if _, rows, err := read(t, long, tabular.FormatXLSX, tabular.DefaultLimits()); err != nil || len(rows) != 1 {
		t.Errorf("a 60 KiB attribute value: rows = %v, err = %v, want it read", rows, err)
	}
}

func TestACorruptWorksheetIsAnErrorAndNeverAShorterTable(t *testing.T) {
	const name = "xl/worksheets/sheet1.xml"
	intact := deflatedPart(t, name, func(emit func(string)) { emit(string(rosterSheet(1500))) })
	damaged := intact
	damaged.stored = bytes.Clone(intact.stored)
	damaged.stored[len(damaged.stored)/2] ^= 0xFF
	cut := intact
	cut.stored = intact.stored[:len(intact.stored)/2]
	wrongSum := intact
	wrongSum.crc ^= 1
	declaresLess, declaresMore := intact, intact
	declaresLess.size--
	declaresMore.size++
	xml := rosterSheet(1500)
	brokenMid := xml[:len(xml)/2]
	cases := map[string]rawPart{
		"a deflate stream with a byte flipped":     damaged,
		"a deflate stream cut in half":             cut,
		"a checksum that does not match":           wrongSum,
		"a size smaller than the data":             declaresLess,
		"a size larger than the data":              declaresMore,
		"xml that stops in the middle of the rows": storedPart(name, brokenMid),
		"an element closed by the wrong tag":       storedPart(name, bytes.Replace(xml, []byte("</row>"), []byte("</cow>"), 700)),
	}

	_, rows, err := read(t, archiveRaw(t, workbookParts(nil), intact), tabular.FormatXLSX, tabular.DefaultLimits())
	if err != nil || len(rows) != 1500 {
		t.Fatalf("the intact sheet: %d rows, err = %v, want 1500 and none", len(rows), err)
	}
	for label, part := range cases {
		t.Run(label, func(t *testing.T) {
			_, rows, err := read(t, archiveRaw(t, workbookParts(nil), part), tabular.FormatXLSX, tabular.DefaultLimits())

			if !errors.Is(err, tabular.ErrFormat) {
				t.Errorf("err = %v with %d rows, want ErrFormat and no table", err, len(rows))
			}
		})
	}
}

func TestACorruptSharedStringsOrStylesPartIsAnError(t *testing.T) {
	sheet := row(1, sharedString("A1", 0))
	parts := map[string]entry{
		"shared strings that stop short":  {name: "xl/sharedStrings.xml", body: `<sst xmlns="` + spreadsheetNS + `"><si><t>name</t></si><si><t>Tr`},
		"styles with a stray closing tag": {name: "xl/styles.xml", body: `<styleSheet xmlns="` + spreadsheetNS + `"><fonts></fills></styleSheet>`},
	}

	for label, part := range parts {
		t.Run(label, func(t *testing.T) {
			entries := workbookParts([]string{"name"})
			for i, e := range entries {
				if e.name == part.name {
					entries[i] = part
				}
			}

			_, _, err := read(t, archive(t, append(entries, sheetEntry(sheet))...), tabular.FormatXLSX, tabular.DefaultLimits())

			if !errors.Is(err, tabular.ErrFormat) {
				t.Errorf("err = %v, want ErrFormat", err)
			}
		})
	}
}

func TestAWorksheetCannotBeHiddenUnderAnotherPartName(t *testing.T) {
	hidden := func(target string) []entry {
		entries := workbookParts(nil)
		for i, e := range entries {
			if e.name == "xl/_rels/workbook.xml.rels" {
				entries[i].body = strings.Replace(e.body, `Target="worksheets/sheet1.xml"`, `Target="`+target+`"`, 1)
			}
		}
		return entries
	}

	for name, tc := range map[string]struct {
		target string
		part   string
		want   error
	}{
		"an unusual extension": {"data/rows.dat", "xl/data/rows.dat", tabular.ErrLayout},
		"a media name":         {"media/image1.png", "xl/media/image1.png", tabular.ErrLayout},
		"an xml part":          {"drawings/drawing1.xml", "xl/drawings/drawing1.xml", tabular.ErrPartTooLarge},
	} {
		t.Run(name, func(t *testing.T) {
			data := archiveRaw(t, hidden(tc.target), nestedWorksheet(t, tc.part, 6))
			var err error

			allocated := allocatedDuring(func() { _, _, err = read(t, data, tabular.FormatXLSX, tabular.DefaultLimits()) })

			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
			if allocated > 128<<20 {
				t.Errorf("allocated %d bytes, want the hidden sheet never streamed (six million nested elements cost over 500 MiB)", allocated)
			}
		})
	}
}

func TestADocumentTypeDeclarationInAPartIsRefused(t *testing.T) {
	data := archive(t, append(workbookParts(nil), entry{
		name: "xl/worksheets/sheet1.xml",
		body: `<?xml version="1.0"?><!DOCTYPE worksheet [<!ENTITY a "b">]><worksheet xmlns="` + spreadsheetNS + `"><sheetData>` + rosterRows(1) + `</sheetData></worksheet>`,
	})...)

	_, _, err := read(t, data, tabular.FormatXLSX, tabular.DefaultLimits())

	if !errors.Is(err, tabular.ErrFormat) {
		t.Errorf("err = %v, want ErrFormat", err)
	}
}

func TestAPartDeclaredInAnotherEncodingIsStillRead(t *testing.T) {
	declaration := `<?xml version="1.0" encoding="ISO-8859-1"?>`
	body := sheetEntry(row(1, text("A1", "caf\xe9")) + row(2, text("A2", "cr\xe8me"))).body
	body = declaration + body[strings.Index(body, "<worksheet"):]
	data := archiveRaw(t, workbookParts(nil), storedPart("xl/worksheets/sheet1.xml", []byte(body)))

	header, rows, err := read(t, data, tabular.FormatXLSX, tabular.DefaultLimits())

	if err != nil || len(header) != 1 || header[0] != "café" || len(rows) != 1 || rows[0].Cells[0] != "crème" {
		t.Errorf("header = %q, rows = %v, err = %v, want the sheet read in the encoding it declares", header, rows, err)
	}
}

func TestAPartSavedInUTF16IsAnErrorNotAnEmptySheet(t *testing.T) {
	body := `<?xml version="1.0" encoding="UTF-16"?>` + sheetEntry(row(1, text("A1", "name"))).body[len(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`):]
	encoded := []byte{0xFF, 0xFE}
	for _, unit := range utf16.Encode([]rune(body)) {
		encoded = binary.LittleEndian.AppendUint16(encoded, unit)
	}
	data := archiveRaw(t, workbookParts(nil), storedPart("xl/worksheets/sheet1.xml", encoded))

	_, _, err := read(t, data, tabular.FormatXLSX, tabular.DefaultLimits())

	if !errors.Is(err, tabular.ErrFormat) {
		t.Errorf("err = %v, want ErrFormat", err)
	}
}

func TestNoTemporaryFileIsEverNeeded(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	biggest := workbook(t, denseSheetRows(2001, worksheetSize-64<<10), nil)

	table, err := tabular.ReadTable(bytes.NewReader(biggest), int64(len(biggest)), tabular.FormatXLSX, tabular.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if left, _ := os.ReadDir(tmp); len(left) != 0 {
		t.Errorf("%d temporary files while reading the largest sheet a workbook may hold", len(left))
	}
	if err := table.Close(); err != nil {
		t.Fatal(err)
	}

	over := workbook(t, denseSheetRows(2001, 17<<20), nil)
	_, _, err = read(t, over, tabular.FormatXLSX, tabular.DefaultLimits())
	wantSizeError(t, err, tabular.ErrSheetTooLarge)
	if left, _ := os.ReadDir(tmp); len(left) != 0 {
		t.Errorf("%d temporary files left by a sheet that was refused", len(left))
	}
}

func denseSheetRows(rows, bytes int) string {
	const columns = 20
	cellText := strings.Repeat("x", max(1, ((bytes/rows-21)/columns)-47))
	var sheet strings.Builder
	for r := 1; r <= rows; r++ {
		cells := make([]string, columns)
		for c := range cells {
			cells[c] = text(fmt.Sprintf("%c%d", 'A'+c, r), cellText)
		}
		sheet.WriteString(row(r, cells...))
	}
	return sheet.String()
}

func TestAQuoteInACommentCdataOrInstructionIsNotATagAttribute(t *testing.T) {
	constructs := map[string]string{
		"a comment with an apostrophe":                   `<!-- teacher's file -->`,
		"a comment with a double quote":                  `<!-- a 5" ruler -->`,
		"a comment with a closing angle":                 `<!-- a > b -->`,
		"cdata with an apostrophe":                       `<![CDATA[Nguyen's class]]>`,
		"cdata with a double quote":                      `<![CDATA[a " b]]>`,
		"cdata with a closing angle":                     `<![CDATA[a > b]]>`,
		"an instruction with an apostrophe":              `<?note it's here?>`,
		"an instruction with a closing angle":            `<?note a > b?>`,
		"a comment with single dashes and a quote":       `<!-- it's - fine - ok -->`,
		"no space after the comment opener":              `<!--it's-->`,
		"an arrow inside a comment":                      `<!-- x -> <b a=' -->`,
		"a bracket inside cdata":                         `<![CDATA[ x]> <b a=']]>`,
		"a closing angle then a quote in an instruction": `<?note x > <b a='?>`,
		"a comment then another with a quote":            `<!-- a --><!-- it's -->`,
		"a comment then cdata with a quote":              `<!-- a --><![CDATA[it's]]>`,
		"several in a row":                               `<!-- a'b --><![CDATA[c'd]]><?x e'f?><!-- g'h --><![CDATA[']]><?y "?>`,
	}
	for name, construct := range constructs {
		t.Run(name, func(t *testing.T) {
			data := workbook(t, construct+rosterRows(1500), nil)

			_, rows, err := read(t, data, tabular.FormatXLSX, tabular.DefaultLimits())

			if err != nil || len(rows) != 1500 {
				t.Errorf("rows = %d, err = %v, want the 1500 rows read", len(rows), err)
			}
		})
	}
}

func TestATagAfterACommentCdataOrInstructionIsStillHeldToTheLimit(t *testing.T) {
	long := `<c ` + strings.Repeat(`a="" `, 1<<20/5*8/5) + `/>`
	for name, construct := range map[string]string{
		"a comment":                      `<!-- it's -->`,
		"cdata":                          `<![CDATA[it's]]>`,
		"an instruction":                 `<?note it's?>`,
		"a lone quote in a comment":      `<!-- " -->`,
		"a lone quote in cdata":          `<![CDATA[']]>`,
		"a lone quote in an instruction": `<?note "?>`,
	} {
		t.Run(name, func(t *testing.T) {
			data := workbook(t, row(1, text("A1", "name"))+`<row r="2">`+construct+long+`</row>`, nil)
			var err error

			allocated := allocatedDuring(func() { _, _, err = read(t, data, tabular.FormatXLSX, tabular.DefaultLimits()) })

			if !errors.Is(err, tabular.ErrFormat) {
				t.Fatalf("err = %v, want ErrFormat", err)
			}
			if allocated > 8<<20 {
				t.Errorf("refusing it allocated %d bytes", allocated)
			}
		})
	}
}

func TestAnUnterminatedCommentOrCdataIsAnError(t *testing.T) {
	for name, construct := range map[string]string{
		"a comment": `<!-- it's never closed ` + rosterRows(1500),
		"cdata":     `<![CDATA[it's never closed ` + rosterRows(1500),
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := read(t, workbook(t, construct, nil), tabular.FormatXLSX, tabular.DefaultLimits())

			if !errors.Is(err, tabular.ErrFormat) {
				t.Errorf("err = %v, want ErrFormat", err)
			}
		})
	}
}
