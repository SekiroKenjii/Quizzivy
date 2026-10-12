package tabular_test

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"

	"quizzivy/internal/platform/tabular"
)

func TestACellASpreadsheetWouldRunIsWrittenAsText(t *testing.T) {
	for _, value := range []string{
		`=HYPERLINK("http://evil.example","click")`, "+1", "-2+3", "@SUM(A1)", "\t=1", "\r=1",
		"  =1+1", "\u00A0=1", "\u3000@SUM(A1)", "＝1+1", "＋1", "－2+3", "＠SUM(A1)", " \t=1",
		"=", "-", "+", "@",
	} {
		if got, want := tabular.EscapeCell(value), "'"+value; got != want {
			t.Errorf("EscapeCell(%q) = %q, want %q", value, got, want)
		}
	}
}

func TestAnOrdinaryCellIsWrittenAsItIs(t *testing.T) {
	for _, value := range []string{
		"", " ", "Nguyễn Văn A", "83.3", "0", "a=b", "x+y", "tên-họ", "Bài kiểm tra - Unit 3", "ngày 2026-10-10", "'=1", "email@example.com",
		"・=1", "\n=1",
	} {
		if got := tabular.EscapeCell(value); got != value {
			t.Errorf("EscapeCell(%q) = %q, want it unchanged", value, got)
		}
	}
}

func TestAFileStartsWithTheMarkAndEndsRowsWithCRLF(t *testing.T) {
	var out bytes.Buffer
	file, err := tabular.NewCSV(&out)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Row("Học viên", "Điểm"); err != nil {
		t.Fatal(err)
	}
	if err := file.Row("Lê, Văn \"B\"", "9.5"); err != nil {
		t.Fatal(err)
	}
	if err := file.Flush(); err != nil {
		t.Fatal(err)
	}

	raw := out.String()
	if !strings.HasPrefix(raw, "\xEF\xBB\xBF") {
		t.Fatalf("the file starts %q, want a UTF-8 byte-order mark", raw[:min(len(raw), 8)])
	}
	if want := "Học viên,Điểm\r\n\"Lê, Văn \"\"B\"\"\",9.5\r\n"; strings.TrimPrefix(raw, "\xEF\xBB\xBF") != want {
		t.Errorf("the file reads %q, want %q", strings.TrimPrefix(raw, "\xEF\xBB\xBF"), want)
	}
}

func TestEveryCellOfEveryRowIsEscaped(t *testing.T) {
	var out bytes.Buffer
	file, err := tabular.NewCSV(&out)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Row("=cmd|' /C calc'!A0", "ok"); err != nil {
		t.Fatal(err)
	}
	if err := file.Row("fine", "@SUM(1+1)", "-5"); err != nil {
		t.Fatal(err)
	}
	if err := file.Flush(); err != nil {
		t.Fatal(err)
	}

	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(out.String(), "\xEF\xBB\xBF")))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"'=cmd|' /C calc'!A0", "ok"}, {"fine", "'@SUM(1+1)", "'-5"}}
	if len(records) != len(want) {
		t.Fatalf("%d records, want %d", len(records), len(want))
	}
	for i := range want {
		for j := range want[i] {
			if records[i][j] != want[i][j] {
				t.Errorf("record %d cell %d = %q, want %q", i, j, records[i][j], want[i][j])
			}
		}
	}
}
