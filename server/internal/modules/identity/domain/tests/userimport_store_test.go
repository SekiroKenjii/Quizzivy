package domain_test

import (
	"errors"
	"strings"
	"testing"

	"quizzivy/internal/modules/identity/domain"
)

func TestAFileIsAWorkbookByItsFirstFourBytesAndNeverByItsName(t *testing.T) {
	for name, tc := range map[string]struct {
		file []byte
		want domain.ImportFormat
	}{
		"a zip local header":             {[]byte("PK\x03\x04rest"), domain.ImportXLSX},
		"a header with nothing after it": {[]byte("PK\x03\x04"), domain.ImportXLSX},
		"an empty archive's end record":  {[]byte("PK\x05\x06"), domain.ImportCSV},
		"three bytes of a header":        {[]byte("PK\x03"), domain.ImportCSV},
		"text that starts with PK":       {[]byte("PK,email\n"), domain.ImportCSV},
		"a header":                       {[]byte("full_name,email\n"), domain.ImportCSV},
		"nothing":                        {nil, domain.ImportCSV},
	} {
		if got := domain.SniffImportFormat(tc.file); got != tc.want {
			t.Errorf("%s: got %q, want %q", name, got, tc.want)
		}
	}
}

func TestOnlyStudentAndTeacherAreDefaultRoles(t *testing.T) {
	for role, want := range map[domain.ImportDefaultRole]bool{
		domain.DefaultStudent: true, domain.DefaultTeacher: true,
		"admin": false, "assistant": false, "": false, "Student": false,
	} {
		if got := role.Valid(); got != want {
			t.Errorf("%q valid = %v, want %v", role, got, want)
		}
	}
}

func TestASourceReadsItsFourColumnsAndKeepsEachRowsLine(t *testing.T) {
	source := domain.ImportSource{
		Header: []string{"Email", "ghi chú", "Full_Name", "CLASS"},
		Rows: []domain.ImportCells{
			{Line: 2, Cells: []string{"an@truong.test", "x", "Nguyễn Văn An", "10A"}},
			{Line: 7, Cells: []string{"binh@truong.test"}},
		},
	}
	rows, err := source.ImportRows()
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.ImportRow{
		{Line: 2, FullName: "Nguyễn Văn An", Email: "an@truong.test", Class: "10A"},
		{Line: 7, Email: "binh@truong.test"},
	}
	if len(rows) != len(want) || rows[0] != want[0] || rows[1] != want[1] {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestASourceWhoseHeaderCannotBeUsedFailsWithTheProblemAndNoRows(t *testing.T) {
	source := domain.ImportSource{
		Header: []string{"email", "email"},
		Rows:   []domain.ImportCells{{Line: 2, Cells: []string{"an@truong.test", "an@truong.test"}}},
	}
	rows, err := source.ImportRows()
	var header *domain.ImportHeaderError
	if !errors.As(err, &header) || rows != nil {
		t.Fatalf("got %+v, %v, want a header error and no rows", rows, err)
	}
	if got := header.Problem; len(got.Missing) != 1 || got.Missing[0] != domain.FieldFullName || len(got.Repeated) != 1 || got.Repeated[0] != domain.FieldEmail {
		t.Errorf("problem = %+v, want full_name missing and email repeated", got)
	}
	if strings.Contains(header.Error(), "an@truong.test") {
		t.Errorf("the error text %q carries a cell of the file", header.Error())
	}
}

func TestAJobKeepsOnlyTheLastElementOfAFilesNameClean(t *testing.T) {
	long := strings.Repeat("ấ", domain.MaxImportFilename+40) + ".csv"
	for name, tc := range map[string]struct{ in, want string }{
		"a plain name":                {"ds-lop-10.csv", "ds-lop-10.csv"},
		"a Windows path":              {`C:\Users\gv\Desktop\ds lớp 10.csv`, "ds lớp 10.csv"},
		"a Unix path":                 {"/home/gv/ds.csv", "ds.csv"},
		"a path that climbs":          {"../../etc/passwd", "passwd"},
		"controls and format marks":   {"ds\u0000\u202e\u200b.csv\n", "ds.csv"},
		"decomposed accents":          {"gia\u0301o vien.csv", "giáo vien.csv"},
		"padding":                     {"  ds.csv  ", "ds.csv"},
		"nothing":                     {"", "import"},
		"a folder alone":              {"files/", "import"},
		"only marks":                  {"\u200b\u200e", "import"},
		"a name over the limit":       {long, strings.Repeat("ấ", domain.MaxImportFilename)},
		"the limit counts characters": {strings.Repeat("ấ", domain.MaxImportFilename), strings.Repeat("ấ", domain.MaxImportFilename)},
	} {
		if got := domain.ImportFilename(tc.in); got != tc.want {
			t.Errorf("%s: got %q, want %q", name, got, tc.want)
		}
	}
}
