package domain

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"quizzivy/internal/shared/content"
)

var (
	// ErrUserImportNotFound is an import that does not exist or belongs to another user, which read alike.
	ErrUserImportNotFound = errors.New("user import: not found")
	// ErrImportNothingToCreate is an import whose rows would create no account: the file has none that passed, or every address already belongs to one.
	ErrImportNothingToCreate = errors.New("user import: the file would create no account")
	// ErrImportSheetsOff is a hand-out sheet asked for on a server that cannot seal the passwords it holds.
	ErrImportSheetsOff = errors.New("user import: this server cannot keep a hand-out sheet")
)

// ImportFormat is the kind of file an import reads.
type ImportFormat string

const (
	// ImportCSV is a UTF-8 text file of records.
	ImportCSV ImportFormat = "csv"
	// ImportXLSX is an Office Open XML workbook.
	ImportXLSX ImportFormat = "xlsx"
)

// SniffImportFormat tells a workbook from a CSV file by the bytes it opens
// with, never by its name. A zip archive opens with PK followed by 0x03 0x04;
// nothing that opens like that is a text table.
func SniffImportFormat(file []byte) ImportFormat {
	if len(file) >= 4 && file[0] == 'P' && file[1] == 'K' && file[2] == 3 && file[3] == 4 {
		return ImportXLSX
	}
	return ImportCSV
}

// ImportDefaultRole is the role a row with no role takes, chosen in the dialog.
type ImportDefaultRole string

const (
	// DefaultStudent gives such a row the built-in Student role.
	DefaultStudent ImportDefaultRole = "student"
	// DefaultTeacher gives such a row the built-in Teacher role.
	DefaultTeacher ImportDefaultRole = "teacher"
)

// Valid reports whether r is one of the two choices.
func (r ImportDefaultRole) Valid() bool { return r == DefaultStudent || r == DefaultTeacher }

// ImportCells is one data row of a file as the reader gave it.
type ImportCells struct {
	Line  int
	Cells []string
}

// ImportSource is a file's header and its data rows, read into memory.
type ImportSource struct {
	Header []string
	Rows   []ImportCells
}

// ImportHeaderError says the header cannot be used.
type ImportHeaderError struct {
	Problem ImportHeaderProblem
}

func (e *ImportHeaderError) Error() string {
	return "user import: the header lacks a required column or repeats one"
}

// ImportRows reads the four columns out of the header and returns each data
// row's cells. It fails with *ImportHeaderError when the header lacks
// full_name or email, or names a column twice.
func (s ImportSource) ImportRows() ([]ImportRow, error) {
	cols, problem := ReadImportColumns(s.Header)
	if !problem.Empty() {
		return nil, &ImportHeaderError{Problem: problem}
	}
	rows := make([]ImportRow, len(s.Rows))
	for i, r := range s.Rows {
		rows[i] = NewImportRow(r.Line, r.Cells, cols)
	}
	return rows, nil
}

// ImportFilename is the name a job keeps for a file: the last element of what
// the browser sent, normalised, without control or format characters, and at
// most MaxImportFilename characters. A name with nothing left is "import".
func ImportFilename(name string) string {
	name = strings.ReplaceAll(name, `\`, "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, content.NFC(name))
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > MaxImportFilename {
		name = string([]rune(name)[:MaxImportFilename])
	}
	if name == "" {
		return "import"
	}
	return name
}

// NewImportJob is a job to store: the rows that passed every rule, in file
// order, and how many rows were refused. The job takes the actor of the
// request that carries it.
type NewImportJob struct {
	RequestID     string
	Filename      string
	Format        ImportFormat
	DefaultRoleID string
	Delivery      ImportDelivery
	Rows          []ImportAccepted
	Invalid       int
	Now           time.Time
}

// UserImports stores user imports and answers what judging a file needs to know
// of the accounts and classes.
type UserImports interface {
	// ExistingEmails returns, lower-cased, those of emails that belong to an
	// account of any state. emails are lower-cased already.
	ExistingEmails(ctx context.Context, emails []string) (map[string]struct{}, error)
	// ActiveClasses returns every class that is not archived.
	ActiveClasses(ctx context.Context) ([]ImportClass, error)
	// FindByRequest returns the job createdBy stored for requestID, and false
	// when there is none.
	FindByRequest(ctx context.Context, createdBy, requestID string) (ImportJob, bool, error)
	// Find returns the job id of createdBy, and ErrUserImportNotFound for any
	// other.
	Find(ctx context.Context, createdBy, id string) (ImportJob, error)
	// Create stores the job and all its rows in one transaction, and audits
	// user_import.requested. A job of the same actor and request id already
	// stored comes back instead, with replayed true, having written nothing.
	// Rows that name an existing address are stored settled as skipped.
	Create(ctx context.Context, req WriteRequest, in NewImportJob) (job ImportJob, replayed bool, err error)
	// TakeSheet hands the importer the passwords of the accounts job id
	// created, once. In one transaction it locks the job, opens each sealed
	// password with open, erases them all, records the download at now and
	// audits user_import.sheet_downloaded. It fails with ErrUserImportNotFound
	// for a job that does not exist or is another user's, with
	// ErrSheetUnavailable for one with no sheet to give at now or none of whose
	// sealed passwords is left, and with open's error, having changed nothing.
	TakeSheet(ctx context.Context, req WriteRequest, id string, now time.Time, open SecretOpener) ([]SheetRow, error)
	// EraseExpiredSheets clears the sealed passwords of at most limit rows of
	// the sheet jobs whose sheet expired at or before now without being
	// downloaded, and returns how many it cleared. It leaves the rows, their
	// outcomes and the jobs as they are, and skips a job whose sheet is being
	// taken, which the next call finds again.
	EraseExpiredSheets(ctx context.Context, now time.Time, limit int) (int, error)
}
