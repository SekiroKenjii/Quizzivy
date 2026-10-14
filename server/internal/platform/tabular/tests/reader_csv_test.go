package tabular_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"quizzivy/internal/platform/tabular"
)

func TestACSVFileGivesItsHeaderAndEachRowWithItsLine(t *testing.T) {
	data := "\xEF\xBB\xBFfull_name, email ,role,class\r\n" +
		"Nguyễn Văn A,a@example.com,student,10A1\r\n" +
		"\r\n" +
		"Lê Thị B,b@example.com,,\r\n"

	header, rows, err := read(t, []byte(data), tabular.FormatCSV, tabular.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}

	if want := []string{"full_name", "email", "role", "class"}; !reflect.DeepEqual(header, want) {
		t.Errorf("header = %q, want %q (the byte-order mark and the cell's spaces gone)", header, want)
	}
	want := []tabular.Row{
		{Line: 2, Cells: []string{"Nguyễn Văn A", "a@example.com", "student", "10A1"}},
		{Line: 4, Cells: []string{"Lê Thị B", "b@example.com", "", ""}},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %v, want %v", rows, want)
	}
}

func TestACSVFileThatUsesSemicolonsIsReadByTheHeaderLine(t *testing.T) {
	data := "full_name;email;note\n" +
		"Trần C;c@example.com;\"điểm 2,5; tốt\"\n" +
		"\"Phạm, D\";d@example.com;ok\n"

	header, rows, err := read(t, []byte(data), tabular.FormatCSV, tabular.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}

	if want := []string{"full_name", "email", "note"}; !reflect.DeepEqual(header, want) {
		t.Errorf("header = %q, want %q", header, want)
	}
	want := [][]string{{"Trần C", "c@example.com", "điểm 2,5; tốt"}, {"Phạm, D", "d@example.com", "ok"}}
	for i, w := range want {
		if i >= len(rows) || !reflect.DeepEqual(rows[i].Cells, w) {
			t.Errorf("row %d = %v, want %q", i, rows, w)
		}
	}
}

func TestADelimiterInsideQuotesInTheHeaderDoesNotChooseTheSeparator(t *testing.T) {
	data := "\"name, full, legal, short\";\"email\"\nA;b@example.com\n"

	header, rows, err := read(t, []byte(data), tabular.FormatCSV, tabular.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}

	if want := []string{"name, full, legal, short", "email"}; !reflect.DeepEqual(header, want) {
		t.Errorf("header = %q, want %q", header, want)
	}
	if len(rows) != 1 || !reflect.DeepEqual(rows[0].Cells, []string{"A", "b@example.com"}) {
		t.Errorf("rows = %v, want one row A, b@example.com", rows)
	}
}

func TestOnlyTheHeaderLineChoosesTheSeparator(t *testing.T) {
	data := "name,note\nA,x;y;z;w\nB,p;q\n"

	header, rows, err := read(t, []byte(data), tabular.FormatCSV, tabular.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}

	if want := []string{"name", "note"}; !reflect.DeepEqual(header, want) {
		t.Errorf("header = %q, want %q", header, want)
	}
	if len(rows) != 2 || rows[0].Cells[1] != "x;y;z;w" {
		t.Errorf("rows = %v, want the semicolons kept inside the note", rows)
	}
}

func TestQuotedFieldsFollowRFC4180(t *testing.T) {
	data := "a,b\r\n\"x, \"\"y\"\"\",\"two\r\nlines\"\r\nnext,row\r\n"

	_, rows, err := read(t, []byte(data), tabular.FormatCSV, tabular.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}

	want := []tabular.Row{
		{Line: 2, Cells: []string{`x, "y"`, "two\nlines"}},
		{Line: 4, Cells: []string{"next", "row"}},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %v, want %v", rows, want)
	}
}

func TestACellThatLooksLikeAFormulaIsReturnedAsTyped(t *testing.T) {
	data := "name,email\n" + `"=HYPERLINK(""http://x"")",+1@example.com` + "\n"

	_, rows, err := read(t, []byte(data), tabular.FormatCSV, tabular.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}

	if want := []string{`=HYPERLINK("http://x")`, "+1@example.com"}; len(rows) != 1 || !reflect.DeepEqual(rows[0].Cells, want) {
		t.Errorf("rows = %v, want one row %q", rows, want)
	}
}

func TestRowsAreWidenedToTheHeaderAndAnExtraCellIsKept(t *testing.T) {
	data := "a,b,c\n1\n2,,,\n3,4,5,6\n"

	_, rows, err := read(t, []byte(data), tabular.FormatCSV, tabular.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}

	want := [][]string{{"1", "", ""}, {"2", "", ""}, {"3", "4", "5", "6"}}
	if len(rows) != len(want) {
		t.Fatalf("rows = %v, want %d", rows, len(want))
	}
	for i := range want {
		if !reflect.DeepEqual(rows[i].Cells, want[i]) {
			t.Errorf("row %d cells = %q, want %q", i, rows[i].Cells, want[i])
		}
	}
}

func TestBlankRowsAreSkippedAndDoNotCountAgainstTheRowLimit(t *testing.T) {
	data := "a,b\n\n , \n1,2\n,\n3,4\n"
	limits := tabular.DefaultLimits()
	limits.Rows = 2

	_, rows, err := read(t, []byte(data), tabular.FormatCSV, limits)
	if err != nil {
		t.Fatal(err)
	}

	if len(rows) != 2 || rows[0].Line != 4 || rows[1].Line != 6 {
		t.Errorf("rows = %v, want the two on lines 4 and 6", rows)
	}
}

func TestACSVFileInAnotherEncodingIsRefusedWithTheWayToSaveItAgain(t *testing.T) {
	for name, data := range map[string][]byte{
		"windows-1258": []byte("full_name,email\nNguy\xEAn V\xE4n A,a@example.com\n"),
		"utf-16 le":    []byte("\xFF\xFEf\x00u\x00l\x00l\x00,\x00e\x00\n\x00"),
		"utf-16 be":    []byte("\xFE\xFF\x00f\x00u\x00l\x00l\x00,\x00e\x00\n"),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := tabular.ReadTable(strings.NewReader(string(data)), int64(len(data)), tabular.FormatCSV, tabular.DefaultLimits())

			if !errors.Is(err, tabular.ErrNotUTF8) {
				t.Fatalf("err = %v, want ErrNotUTF8", err)
			}
			if !strings.Contains(err.Error(), `"CSV UTF-8"`) {
				t.Errorf("message %q does not say to save it as \"CSV UTF-8\"", err)
			}
		})
	}
}

func TestAMalformedQuoteIsNotAReadableTable(t *testing.T) {
	data := "a,b\n1,\"unterminated\n"

	_, _, err := read(t, []byte(data), tabular.FormatCSV, tabular.DefaultLimits())

	if !errors.Is(err, tabular.ErrFormat) {
		t.Errorf("err = %v, want ErrFormat", err)
	}
}

func TestAFileWithNoHeaderRowIsEmpty(t *testing.T) {
	for name, data := range map[string]string{
		"nothing":      "",
		"only the BOM": "\xEF\xBB\xBF",
		"blank lines":  "\n\r\n , ,\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := read(t, []byte(data), tabular.FormatCSV, tabular.DefaultLimits())

			if !errors.Is(err, tabular.ErrEmpty) {
				t.Errorf("err = %v, want ErrEmpty", err)
			}
		})
	}
}

func TestTheRowLimitAdmitsExactlyThatManyRows(t *testing.T) {
	build := func(rows int) []byte {
		var b strings.Builder
		b.WriteString("a,b\n")
		for i := range rows {
			fmt.Fprintf(&b, "%d,x\n", i)
		}
		return []byte(b.String())
	}

	_, rows, err := read(t, build(2000), tabular.FormatCSV, tabular.DefaultLimits())
	if err != nil || len(rows) != 2000 {
		t.Fatalf("2000 rows: got %d rows, err %v", len(rows), err)
	}

	_, rows, err = read(t, build(2001), tabular.FormatCSV, tabular.DefaultLimits())
	if !errors.Is(err, tabular.ErrTooManyRows) {
		t.Fatalf("2001 rows: err = %v, want ErrTooManyRows", err)
	}
	if len(rows) != 2000 {
		t.Errorf("2001 rows: %d rows came before the error, want 2000", len(rows))
	}
}

func TestTheColumnLimitIgnoresEmptyCellsAtTheEndOfARow(t *testing.T) {
	wide := strings.Repeat("c,", 20) + "c"
	padded := "a,b" + strings.Repeat(",", 40)

	if _, _, err := read(t, []byte(strings.Repeat("c,", 19)+"c\n"+padded+"\n"), tabular.FormatCSV, tabular.DefaultLimits()); err != nil {
		t.Errorf("20 columns and trailing commas: %v", err)
	}
	if _, _, err := read(t, []byte(wide+"\n"), tabular.FormatCSV, tabular.DefaultLimits()); !errors.Is(err, tabular.ErrTooManyColumns) {
		t.Errorf("a 21-column header: err = %v, want ErrTooManyColumns", err)
	}
	if _, _, err := read(t, []byte("a,b\n"+wide+"\n"), tabular.FormatCSV, tabular.DefaultLimits()); !errors.Is(err, tabular.ErrTooManyColumns) {
		t.Errorf("a 21-column row: err = %v, want ErrTooManyColumns", err)
	}
}

func TestTheCellLimitCountsCharactersNotBytes(t *testing.T) {
	atLimit := strings.Repeat("ế", 1000)

	if _, _, err := read(t, []byte("a\n"+atLimit+"\n"), tabular.FormatCSV, tabular.DefaultLimits()); err != nil {
		t.Errorf("1000 characters: %v", err)
	}
	if _, _, err := read(t, []byte("a\n"+atLimit+"x\n"), tabular.FormatCSV, tabular.DefaultLimits()); !errors.Is(err, tabular.ErrCellTooLong) {
		t.Errorf("1001 characters: err = %v, want ErrCellTooLong", err)
	}
	if _, _, err := read(t, []byte(atLimit+"x\n1\n"), tabular.FormatCSV, tabular.DefaultLimits()); !errors.Is(err, tabular.ErrCellTooLong) {
		t.Errorf("a 1001-character header cell: err = %v, want ErrCellTooLong", err)
	}
}

func TestAFileLargerThanTheByteLimitIsRefusedBeforeItIsRead(t *testing.T) {
	limits := tabular.DefaultLimits()
	limits.Bytes = 16

	for _, format := range []tabular.Format{tabular.FormatCSV, tabular.FormatXLSX} {
		_, _, err := read(t, []byte("a,b\n1,2\n3,4,5,6,7\n"), format, limits)

		if !errors.Is(err, tabular.ErrTooLarge) {
			t.Errorf("%s: err = %v, want ErrTooLarge", format, err)
		}
	}
}

func TestALimitThatIsNotPositiveIsRefused(t *testing.T) {
	for name, mutate := range map[string]func(*tabular.Limits){
		"bytes":   func(l *tabular.Limits) { l.Bytes = 0 },
		"rows":    func(l *tabular.Limits) { l.Rows = 0 },
		"columns": func(l *tabular.Limits) { l.Columns = -1 },
		"cells":   func(l *tabular.Limits) { l.CellChars = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			limits := tabular.DefaultLimits()
			mutate(&limits)

			if _, _, err := read(t, []byte("a\n1\n"), tabular.FormatCSV, limits); err == nil {
				t.Error("ReadTable accepted the limit")
			}
		})
	}
}

func TestAnUnknownFormatIsRefused(t *testing.T) {
	if _, _, err := read(t, []byte("a\n1\n"), tabular.Format("ods"), tabular.DefaultLimits()); err == nil {
		t.Error("ReadTable read an ods file")
	}
}

func TestStoppingEarlyAndClosingTwiceAreSafe(t *testing.T) {
	data := []byte("a\n1\n2\n3\n")
	table, err := tabular.ReadTable(strings.NewReader(string(data)), int64(len(data)), tabular.FormatCSV, tabular.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}

	seen := 0
	for _, err := range table.Rows() {
		if err != nil {
			t.Fatal(err)
		}
		seen++
		break
	}
	for _, err := range table.Rows() {
		if err != nil {
			t.Fatal(err)
		}
		seen++
	}

	if seen != 3 {
		t.Errorf("saw %d rows across a break and a second pass, want 3 (it continues where it stopped)", seen)
	}
	if err := table.Close(); err != nil {
		t.Error(err)
	}
	if err := table.Close(); err != nil {
		t.Error(err)
	}
}
