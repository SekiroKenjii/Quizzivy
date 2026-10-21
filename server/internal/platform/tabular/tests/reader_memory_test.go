package tabular_test

import (
	"bytes"
	"encoding/base64"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"quizzivy/internal/platform/tabular"
)

func TestReadingAFiveMiBWorkbookStaysWithinMemory(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	book := excelize.NewFile()
	stream, err := book.NewStreamWriter("Sheet1")
	if err != nil {
		t.Fatal(err)
	}
	raw := make([]byte, 114)
	for r := 1; r <= 2001; r++ {
		cells := make([]any, 20)
		for c := range cells {
			for i := range raw {
				raw[i] = byte(rng.UintN(256))
			}
			cells[c] = base64.StdEncoding.EncodeToString(raw)
		}
		cell, err := excelize.CoordinatesToCellName(1, r)
		if err != nil {
			t.Fatal(err)
		}
		if err := stream.SetRow(cell, cells); err != nil {
			t.Fatal(err)
		}
	}
	if err := stream.Flush(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := book.Write(&out); err != nil {
		t.Fatal(err)
	}
	if size := out.Len(); size < 4<<20 || size > 5<<20 {
		t.Fatalf("the workbook is %d bytes, want one near the 5 MiB upload limit", size)
	}
	var rows int

	allocated := allocatedDuring(func() {
		_, got, err := read(t, out.Bytes(), tabular.FormatXLSX, tabular.DefaultLimits())
		if err != nil {
			t.Error(err)
		}
		rows = len(got)
	})

	if rows != 2000 {
		t.Errorf("read %d rows, want 2000", rows)
	}
	t.Logf("a %.1f MiB workbook of 2000 rows and 20 columns allocated %.1f MiB to read", float64(out.Len())/(1<<20), float64(allocated)/(1<<20))
	if allocated > 256<<20 {
		t.Errorf("reading it allocated %d bytes, want under 256 MiB", allocated)
	}
}

func TestTheLargestWorkbookTheLimitsAdmitStaysWithinMemory(t *testing.T) {
	styles := `<styleSheet xmlns="` + spreadsheetNS + `"><cellXfs>` + strings.Repeat("<xf/>", 1<<20/5-60) + `</cellXfs></styleSheet>`
	calcChain := `<calcChain xmlns="` + spreadsheetNS + `">` + strings.Repeat(`<c r="A1" i="1"/>`, 1<<20/17-20) + `</calcChain>`
	shared := `<sst xmlns="` + spreadsheetNS + `">` + strings.Repeat("<si><t>x</t></si>", 2<<20/17-20) + `</sst>`
	sheet := denseSheetRows(2001, worksheetSize-64<<10)
	data := workbook(t, sheet, nil,
		entry{name: "xl/styles.xml", body: styles},
		entry{name: "xl/calcChain.xml", body: calcChain},
		entry{name: "xl/sharedStrings.xml", body: shared})
	var (
		err  error
		rows []tabular.Row
	)

	allocated := allocatedDuring(func() { _, rows, err = read(t, data, tabular.FormatXLSX, tabular.DefaultLimits()) })

	if err != nil || len(rows) != 2000 {
		t.Fatalf("a workbook at the part limits: %d rows, err = %v, want 2000 rows read", len(rows), err)
	}
	t.Logf("a %d byte workbook whose sheet, styles, calc chain and shared strings are at their limits allocated %.1f MiB to read", len(data), float64(allocated)/(1<<20))
	if allocated > 768<<20 {
		t.Errorf("reading it allocated %d bytes, want under 768 MiB", allocated)
	}
}
