package tabular_test

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"quizzivy/internal/platform/tabular"
)

func TestAWorkbookGivesItsFirstSheetAsHeaderAndRows(t *testing.T) {
	book := excelize.NewFile()
	if _, err := book.NewSheet("Other"); err != nil {
		t.Fatal(err)
	}
	for cell, value := range map[string]any{"A1": "full_name", "B1": "email", "A2": "Nguyễn Văn A", "B2": "a@example.com", "A3": "Lê Thị B", "B3": true} {
		if err := book.SetCellValue("Sheet1", cell, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := book.SetCellValue("Other", "A1", "ignored"); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := book.Write(&out); err != nil {
		t.Fatal(err)
	}

	header, rows, err := read(t, out.Bytes(), tabular.FormatXLSX, tabular.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}

	if want := []string{"full_name", "email"}; !reflect.DeepEqual(header, want) {
		t.Errorf("header = %q, want %q", header, want)
	}
	want := []tabular.Row{
		{Line: 2, Cells: []string{"Nguyễn Văn A", "a@example.com"}},
		{Line: 3, Cells: []string{"Lê Thị B", "TRUE"}},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %v, want %v", rows, want)
	}
}

func TestSharedStringsInlineStringsAndNumbersAreReadAsText(t *testing.T) {
	data := workbook(t,
		row(1, sharedString("A1", 0), sharedString("B1", 1))+
			row(2, sharedString("A2", 2), text("B2", "inline &amp; ok"), `<c r="C2"><v>42</v></c>`)+
			row(3, `<c r="A3" t="b"><v>1</v></c>`, `<c r="B3" t="e"><v>#DIV/0!</v></c>`, `<c r="C3" s="1"><v>46309</v></c>`),
		[]string{"full_name", "email", "Trần Văn C"})

	header, rows, err := read(t, data, tabular.FormatXLSX, tabular.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}

	if want := []string{"full_name", "email"}; !reflect.DeepEqual(header, want) {
		t.Errorf("header = %q, want %q", header, want)
	}
	want := []tabular.Row{
		{Line: 2, Cells: []string{"Trần Văn C", "inline & ok", "42"}},
		{Line: 3, Cells: []string{"TRUE", "#DIV/0!", "2026-10-14"}},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %v, want %v", rows, want)
	}
}

func TestAFormulaCellYieldsItsCachedValueAndIsNeverEvaluated(t *testing.T) {
	data := workbook(t,
		row(1, text("A1", "name"), text("B1", "email"))+
			row(2,
				`<c r="A2"><f>1+1</f><v>99</v></c>`,
				`<c r="B2" t="str"><f>HYPERLINK("http://x","click")</f><v>click</v></c>`,
				`<c r="C2"><f>SUM(A2:B2)</f></c>`,
				text("D2", "after")),
		nil)

	_, rows, err := read(t, data, tabular.FormatXLSX, tabular.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"99", "click", "", "after"}
	if len(rows) != 1 || !reflect.DeepEqual(rows[0].Cells, want) {
		t.Errorf("rows = %v, want one row %q", rows, want)
	}
}

func TestBlankRowsAndGapsInAWorkbookAreSkippedWithTheRowNumbersKept(t *testing.T) {
	data := workbook(t,
		row(1, text("A1", "name"))+
			row(2, `<c r="A2"/>`)+
			row(5, text("A5", "first"))+
			row(6, text("A6", "  "))+
			row(9, text("A9", "second")),
		nil)

	_, rows, err := read(t, data, tabular.FormatXLSX, tabular.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}

	if len(rows) != 2 || rows[0].Line != 5 || rows[1].Line != 9 || rows[1].Cells[0] != "second" {
		t.Errorf("rows = %v, want the rows numbered 5 and 9", rows)
	}
}

func TestAWorkbookIsHeldToTheRowColumnAndCellLimits(t *testing.T) {
	build := func(rows int) []byte {
		var sheet strings.Builder
		sheet.WriteString(row(1, text("A1", "n"), text("B1", "e")))
		for i := 2; i <= rows+1; i++ {
			sheet.WriteString(row(i, text(fmt.Sprintf("A%d", i), "x"), text(fmt.Sprintf("B%d", i), "y")))
		}
		return workbook(t, sheet.String(), nil)
	}

	_, rows, err := read(t, build(2000), tabular.FormatXLSX, tabular.DefaultLimits())
	if err != nil || len(rows) != 2000 {
		t.Fatalf("2000 rows: got %d rows, err %v", len(rows), err)
	}
	if _, rows, err = read(t, build(2001), tabular.FormatXLSX, tabular.DefaultLimits()); !errors.Is(err, tabular.ErrTooManyRows) || len(rows) != 2000 {
		t.Errorf("2001 rows: %d rows, err %v, want 2000 and ErrTooManyRows", len(rows), err)
	}

	wide := workbook(t, row(1, text("A1", "n"))+row(2, text("U2", "twenty-first")), nil)
	if _, _, err := read(t, wide, tabular.FormatXLSX, tabular.DefaultLimits()); !errors.Is(err, tabular.ErrTooManyColumns) {
		t.Errorf("a row reaching column U: err = %v, want ErrTooManyColumns", err)
	}

	long := workbook(t, row(1, text("A1", "n"))+row(2, text("A2", strings.Repeat("ế", 1001))), nil)
	if _, _, err := read(t, long, tabular.FormatXLSX, tabular.DefaultLimits()); !errors.Is(err, tabular.ErrCellTooLong) {
		t.Errorf("a 1001-character cell: err = %v, want ErrCellTooLong", err)
	}
}

func TestAnArchiveThatExpandsPastTheLimitIsRefusedBeforeExcelizeAllocates(t *testing.T) {
	bomb := archiveOfBombs(t, []string{"xl/worksheets/sheet1.xml"}, 96<<20)
	if len(bomb) > 1<<20 {
		t.Fatalf("the test bomb is %d bytes, want it small enough to be under any upload limit", len(bomb))
	}
	var err error

	allocated := allocatedDuring(func() { _, _, err = read(t, bomb, tabular.FormatXLSX, tabular.DefaultLimits()) })

	if !errors.Is(err, tabular.ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
	if allocated > 8<<20 {
		t.Errorf("refusing the bomb allocated %d bytes, want the 96 MiB it expands to left alone", allocated)
	}
}

func TestSeveralEntriesThatTogetherExpandPastTheLimitAreRefused(t *testing.T) {
	bomb := archiveOfBombs(t, []string{"xl/a.bin", "xl/b.bin", "xl/c.bin", "xl/d.bin"}, 9<<20)

	_, _, err := read(t, bomb, tabular.FormatXLSX, tabular.DefaultLimits())

	if !errors.Is(err, tabular.ErrTooLarge) {
		t.Errorf("err = %v, want ErrTooLarge (4 x 9 MiB is over 32 MiB although each entry is under it)", err)
	}
}

func TestAHeaderThatDeclaresAHugeEntryIsRefusedWithoutBelievingItOrAllocatingIt(t *testing.T) {
	for name, declared := range map[string]uint64{"1 GiB": 1 << 30, "the largest size": math.MaxUint64} {
		t.Run(name, func(t *testing.T) {
			data := archiveDeclaring(t, "xl/worksheets/sheet1.xml", declared, 0)
			var err error

			allocated := allocatedDuring(func() { _, _, err = read(t, data, tabular.FormatXLSX, tabular.DefaultLimits()) })

			if !errors.Is(err, tabular.ErrTooLarge) {
				t.Fatalf("err = %v, want ErrTooLarge", err)
			}
			if allocated > 8<<20 {
				t.Errorf("allocated %d bytes for an entry that declares %d", allocated, declared)
			}
		})
	}
}

func TestAnArchiveWithMoreThanAThousandEntriesIsRefused(t *testing.T) {
	entries := make([]entry, 1001)
	for i := range entries {
		entries[i] = entry{name: fmt.Sprintf("xl/part%04d.xml", i)}
	}

	_, _, err := read(t, archive(t, entries...), tabular.FormatXLSX, tabular.DefaultLimits())

	if !errors.Is(err, tabular.ErrTooLarge) {
		t.Errorf("err = %v, want ErrTooLarge", err)
	}
}

func TestAnXMLPartAWorkbookDecodesWholeIsHeldToItsOwnLimit(t *testing.T) {
	cells := strings.Repeat("<xf/>", 3<<20)
	for name, extra := range map[string]entry{
		"styles":         {name: "xl/styles.xml", body: `<styleSheet xmlns="` + spreadsheetNS + `"><cellXfs>` + cells + `</cellXfs></styleSheet>`},
		"calc chain":     {name: "xl/calcChain.xml", body: `<calcChain xmlns="` + spreadsheetNS + `">` + strings.Repeat(`<c r="A1" i="1"/>`, 100000) + `</calcChain>`},
		"shared strings": {name: "xl/sharedStrings.xml", body: `<sst xmlns="` + spreadsheetNS + `">` + strings.Repeat("<si><t>x</t></si>", 140000) + `</sst>`},
	} {
		t.Run(name, func(t *testing.T) {
			data := workbook(t, row(1, text("A1", "n"))+row(2, text("A2", "v")), nil, extra)
			if len(data) > 1<<20 {
				t.Fatalf("the test workbook is %d bytes, want a small file that expands", len(data))
			}
			var err error

			allocated := allocatedDuring(func() { _, _, err = read(t, data, tabular.FormatXLSX, tabular.DefaultLimits()) })

			if !errors.Is(err, tabular.ErrTooLarge) {
				t.Fatalf("err = %v, want ErrTooLarge", err)
			}
			if allocated > 8<<20 {
				t.Errorf("refusing it allocated %d bytes, want the decoding it would have caused left undone", allocated)
			}
		})
	}
}

func TestPartsThatAreNotDecodedWholeAreOnlyHeldToTheTotal(t *testing.T) {
	data := workbook(t, row(1, text("A1", "n"))+row(2, text("A2", "v")), nil,
		entry{name: "xl/media/logo.png", body: strings.Repeat("\x89", 3<<20)},
		entry{name: "xl/drawings/drawing1.bin", body: strings.Repeat("d", 2<<20)})

	_, rows, err := read(t, data, tabular.FormatXLSX, tabular.DefaultLimits())
	if err != nil || len(rows) != 1 {
		t.Errorf("rows = %v, err = %v, want the workbook with its logo read", rows, err)
	}
}

func TestAWorkbookWithMacrosIsRefused(t *testing.T) {
	for name, extra := range map[string]entry{
		"vbaProject":       {name: "xl/vbaProject.bin", body: "macro"},
		"any case":         {name: "XL/VBAPROJECT.BIN", body: "macro"},
		"backslashes":      {name: `xl\vbaProject.bin`, body: "macro"},
		"an Excel 4 macro": {name: "xl/macrosheets/sheet1.xml", body: "<xm/>"},
	} {
		t.Run(name, func(t *testing.T) {
			data := workbook(t, row(1, text("A1", "n"))+row(2, text("A2", "v")), nil, extra)

			_, _, err := read(t, data, tabular.FormatXLSX, tabular.DefaultLimits())

			if !errors.Is(err, tabular.ErrMacros) {
				t.Errorf("err = %v, want ErrMacros", err)
			}
		})
	}
}

func TestAnEncryptedWorkbookIsRefused(t *testing.T) {
	book := excelize.NewFile()
	if err := book.SetCellValue("Sheet1", "A1", "secret"); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := book.Write(&out, excelize.Options{Password: "hunter2"}); err != nil {
		t.Fatal(err)
	}

	_, _, err := read(t, out.Bytes(), tabular.FormatXLSX, tabular.DefaultLimits())

	if !errors.Is(err, tabular.ErrEncrypted) {
		t.Errorf("err = %v, want ErrEncrypted", err)
	}
}

func TestAnArchiveEntryThatIsEncryptedIsRefused(t *testing.T) {
	data := archiveDeclaring(t, "xl/workbook.xml", 4, 0x1)

	_, _, err := read(t, data, tabular.FormatXLSX, tabular.DefaultLimits())

	if !errors.Is(err, tabular.ErrEncrypted) {
		t.Errorf("err = %v, want ErrEncrypted", err)
	}
}

func TestAnOldExcelFileIsNotAReadableWorkbook(t *testing.T) {
	data := append([]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}, make([]byte, 512)...)

	_, _, err := read(t, data, tabular.FormatXLSX, tabular.DefaultLimits())

	if !errors.Is(err, tabular.ErrFormat) || errors.Is(err, tabular.ErrEncrypted) {
		t.Errorf("err = %v, want ErrFormat and not ErrEncrypted", err)
	}
}

func TestFilesThatAreNotWorkbooksAreNotReadableTables(t *testing.T) {
	for name, data := range map[string][]byte{
		"text":                     []byte("name,email\nA,b@example.com\n"),
		"a truncated archive":      workbook(t, row(1, text("A1", "n")), nil)[:200],
		"an archive with no sheet": archive(t, entry{name: "word/document.xml", body: "<w/>"}),
		"two bytes":                {0x50, 0x4B},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := read(t, data, tabular.FormatXLSX, tabular.DefaultLimits())

			if !errors.Is(err, tabular.ErrFormat) {
				t.Errorf("err = %v, want ErrFormat", err)
			}
		})
	}
}

func TestAWorkbookWithNoRowsHasNoHeader(t *testing.T) {
	_, _, err := read(t, workbook(t, "", nil), tabular.FormatXLSX, tabular.DefaultLimits())

	if !errors.Is(err, tabular.ErrEmpty) {
		t.Errorf("err = %v, want ErrEmpty", err)
	}
}

func TestADateTypedIntoAWorkbookIsReadAsTheUserSawIt(t *testing.T) {
	book := excelize.NewFile()
	style, err := book.NewStyle(&excelize.Style{CustomNumFmt: ptr("yyyy-mm-dd")})
	if err != nil {
		t.Fatal(err)
	}
	if err := book.SetCellValue("Sheet1", "A1", "joined"); err != nil {
		t.Fatal(err)
	}
	if err := book.SetCellValue("Sheet1", "A2", time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if err := book.SetCellStyle("Sheet1", "A2", "A2", style); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := book.Write(&out); err != nil {
		t.Fatal(err)
	}

	_, rows, err := read(t, out.Bytes(), tabular.FormatXLSX, tabular.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}

	if len(rows) != 1 || rows[0].Cells[0] != "2026-10-14" {
		t.Errorf("rows = %v, want the date as 2026-10-14", rows)
	}
}

func ptr[T any](v T) *T { return &v }
