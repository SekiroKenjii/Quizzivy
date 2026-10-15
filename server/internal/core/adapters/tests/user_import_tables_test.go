package adapters_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"quizzivy/internal/core/adapters"
	identitydomain "quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/tabular"
)

func workbookOf(t *testing.T, rows ...[]any) []byte {
	t.Helper()
	book := excelize.NewFile()
	defer func() { _ = book.Close() }()
	sheet := book.GetSheetName(0)
	for i, row := range rows {
		cell, err := excelize.CoordinatesToCellName(1, i+1)
		if err != nil {
			t.Fatal(err)
		}
		if err := book.SetSheetRow(sheet, cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	buffer, err := book.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestAUserImportCSVGivesItsHeaderAndEachRowWithItsLine(t *testing.T) {
	file := []byte("\xef\xbb\xbffull_name;email;role\r\nNguyễn Văn An;an@truong.test;\r\n\r\nTrần Thị Bình;binh@truong.test;giáo viên\r\n")

	source, err := adapters.UserImportTables{}.Read(file, identitydomain.ImportCSV)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(source.Header, "|"); got != "full_name|email|role" {
		t.Errorf("header = %q", got)
	}
	if len(source.Rows) != 2 || source.Rows[0].Line != 2 || source.Rows[1].Line != 4 ||
		source.Rows[1].Cells[0] != "Trần Thị Bình" || source.Rows[1].Cells[2] != "giáo viên" {
		t.Errorf("rows = %+v, want the two rows on lines 2 and 4 (the blank line skipped, not renumbered)", source.Rows)
	}
}

func TestAUserImportWorkbookGivesTheSameShapeAsACSV(t *testing.T) {
	file := workbookOf(t,
		[]any{"full_name", "email", "role", "class"},
		[]any{"Nguyễn Văn An", "an@truong.test", "student", "Lớp 10A"},
		[]any{"Trần Thị Bình", "binh@truong.test"},
	)

	source, err := adapters.UserImportTables{}.Read(file, identitydomain.ImportXLSX)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := source.ImportRows()
	if err != nil {
		t.Fatal(err)
	}
	want := []identitydomain.ImportRow{
		{Line: 2, FullName: "Nguyễn Văn An", Email: "an@truong.test", Role: "student", Class: "Lớp 10A"},
		{Line: 3, FullName: "Trần Thị Bình", Email: "binh@truong.test"},
	}
	if len(rows) != 2 || rows[0] != want[0] || rows[1] != want[1] {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestAUserImportFileOverTheReadersLimitsIsRefusedWithTheReadersOwnError(t *testing.T) {
	var many strings.Builder
	many.WriteString("full_name,email\n")
	for i := range 2001 {
		fmt.Fprintf(&many, "Học sinh %d,hs%d@truong.test\n", i, i)
	}
	var wide strings.Builder
	wide.WriteString("full_name,email")
	for i := range 25 {
		fmt.Fprintf(&wide, ",c%d", i)
	}
	wide.WriteString("\nA,a@truong.test\n")

	for name, tc := range map[string]struct {
		file   []byte
		format identitydomain.ImportFormat
		want   error
	}{
		"a CSV in another encoding": {[]byte("full_name,email\nA\xff,a@truong.test\n"), identitydomain.ImportCSV, tabular.ErrNotUTF8},
		"no header":                 {nil, identitydomain.ImportCSV, tabular.ErrEmpty},
		"2,001 rows":                {[]byte(many.String()), identitydomain.ImportCSV, tabular.ErrTooManyRows},
		"25 columns":                {[]byte(wide.String()), identitydomain.ImportCSV, tabular.ErrTooManyColumns},
		"a cell of 1,001 characters": {[]byte("full_name,email\n" + strings.Repeat("x", 1001) + ",a@truong.test\n"),
			identitydomain.ImportCSV, tabular.ErrCellTooLong},
		"bytes that open like a zip but are not": {append([]byte("PK\x03\x04"), make([]byte, 64)...), identitydomain.ImportXLSX, tabular.ErrFormat},
	} {
		if _, err := (adapters.UserImportTables{}).Read(tc.file, tc.format); !errors.Is(err, tc.want) {
			t.Errorf("%s: got %v, want %v", name, err, tc.want)
		}
	}
}

func TestAUserImportReadReturnsNoRowsOnceTheReaderFailsPartWay(t *testing.T) {
	file := []byte("full_name,email\nAn,an@truong.test\nBình,\xff\n")
	source, err := adapters.UserImportTables{}.Read(file, identitydomain.ImportCSV)
	if !errors.Is(err, tabular.ErrNotUTF8) || len(source.Rows) != 0 || len(source.Header) != 0 {
		t.Errorf("got %+v, %v, want nothing but the reader's error", source, err)
	}
}
