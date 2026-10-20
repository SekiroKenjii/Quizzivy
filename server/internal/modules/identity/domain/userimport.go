package domain

import (
	"slices"
	"time"
)

const (
	// MaxImportRows is the most data rows one user import reads.
	MaxImportRows = 2000
	// MaxImportIssuesShown is the most refused cells a preview or a created
	// job reports; the count of refused rows is exact.
	MaxImportIssuesShown = 50
	// MaxExistingSample is the most existing addresses a preview names.
	MaxExistingSample = 5
	// MaxImportFilename is the longest file name a job keeps.
	MaxImportFilename = 255
	// MaxEmailLength is the longest address an account takes.
	MaxEmailLength = 254
)

// ImportDelivery is how the accounts of an import first get a way in.
type ImportDelivery string

const (
	// ImportEmailLink emails each new account a link to choose a password.
	ImportEmailLink ImportDelivery = "email_link"
	// ImportSheet gives each new account a temporary password and collects
	// them in a sheet the importer downloads once.
	ImportSheet ImportDelivery = "sheet"
)

// Valid reports whether d is one of the two deliveries.
func (d ImportDelivery) Valid() bool { return d == ImportEmailLink || d == ImportSheet }

// ImportStatus is where a job stands.
type ImportStatus string

const (
	ImportQueued  ImportStatus = "queued"
	ImportRunning ImportStatus = "running"
	ImportDone    ImportStatus = "done"
	ImportFailed  ImportStatus = "failed"
)

// ImportField names a column of an import file.
type ImportField string

const (
	FieldFullName ImportField = "full_name"
	FieldEmail    ImportField = "email"
	FieldRole     ImportField = "role"
	FieldClass    ImportField = "class"
)

// ImportCode is why a cell was refused.
type ImportCode string

const (
	CodeRequired        ImportCode = "REQUIRED"
	CodeTooLong         ImportCode = "TOO_LONG"
	CodeInvalid         ImportCode = "INVALID"
	CodeFormulaLike     ImportCode = "FORMULA_LIKE"
	CodeDuplicateInFile ImportCode = "DUPLICATE_IN_FILE"
	CodeUnknownRole     ImportCode = "UNKNOWN_ROLE"
	CodeAmbiguousRole   ImportCode = "AMBIGUOUS_ROLE"
	CodeRoleNotAllowed  ImportCode = "ROLE_NOT_ALLOWED"
	CodeRoleNotGivable  ImportCode = "ROLE_NOT_GIVABLE"
	CodeUnknownClass    ImportCode = "UNKNOWN_CLASS"
	CodeAmbiguousClass  ImportCode = "AMBIGUOUS_CLASS"
	CodeClassNotAllowed ImportCode = "CLASS_NOT_ALLOWED"
)

// ImportColumns holds where each column of the file's header is, or -1.
type ImportColumns struct {
	FullName, Email, Role, Class int
}

// ImportHeaderProblem lists what is wrong with a header: the required columns
// it lacks and the columns it names twice.
type ImportHeaderProblem struct {
	Missing, Repeated []ImportField
}

// Empty reports whether the header is usable.
func (p ImportHeaderProblem) Empty() bool { return len(p.Missing) == 0 && len(p.Repeated) == 0 }

// ReadImportColumns finds the four columns in header, whatever their case or
// order. full_name and email are required; role and class may be absent, and
// any other column is ignored.
func ReadImportColumns(header []string) (ImportColumns, ImportHeaderProblem) {
	cols := ImportColumns{FullName: -1, Email: -1, Role: -1, Class: -1}
	var problem ImportHeaderProblem
	for i, cell := range header {
		field := ImportField(foldText(cell))
		slot := cols.slot(field)
		switch {
		case slot == nil:
		case *slot >= 0:
			if !slices.Contains(problem.Repeated, field) {
				problem.Repeated = append(problem.Repeated, field)
			}
		default:
			*slot = i
		}
	}
	if cols.FullName < 0 {
		problem.Missing = append(problem.Missing, FieldFullName)
	}
	if cols.Email < 0 {
		problem.Missing = append(problem.Missing, FieldEmail)
	}
	return cols, problem
}

func (c *ImportColumns) slot(field ImportField) *int {
	switch field {
	case FieldFullName:
		return &c.FullName
	case FieldEmail:
		return &c.Email
	case FieldRole:
		return &c.Role
	case FieldClass:
		return &c.Class
	}
	return nil
}

// ImportRow is one data row of the file as its cells read, before any rule.
type ImportRow struct {
	Line                         int
	FullName, Email, Role, Class string
}

// NewImportRow picks the four cells out of a row. A cell the row does not have
// reads empty.
func NewImportRow(line int, cells []string, cols ImportColumns) ImportRow {
	at := func(i int) string {
		if i < 0 || i >= len(cells) {
			return ""
		}
		return cells[i]
	}
	return ImportRow{Line: line, FullName: at(cols.FullName), Email: at(cols.Email), Role: at(cols.Role), Class: at(cols.Class)}
}

// ImportJob is a user import as the importer sees it.
type ImportJob struct {
	ID                string
	CreatedBy         string
	RequestID         string
	Filename          string
	Format            ImportFormat
	DefaultRoleID     string
	Delivery          ImportDelivery
	Status            ImportStatus
	Total             int
	Created           int
	Skipped           int
	Invalid           int
	Attempts          int
	ErrorCode         string
	SheetExpiresAt    *time.Time
	SheetDownloadedAt *time.Time
	CreatedAt         time.Time
	StartedAt         *time.Time
	CompletedAt       *time.Time
}

// Processed is the rows the job has settled: created or skipped.
func (j ImportJob) Processed() int { return j.Created + j.Skipped }

// ImportSheetState is whether the hand-out sheet can be downloaded.
type ImportSheetState struct {
	Available    bool
	ExpiresAt    *time.Time
	DownloadedAt *time.Time
}

// Sheet tells what the importer may do about the hand-out sheet at now. A job
// that is not delivered by sheet has none.
func (j ImportJob) Sheet(now time.Time) (ImportSheetState, bool) {
	if j.Delivery != ImportSheet {
		return ImportSheetState{}, false
	}
	state := ImportSheetState{ExpiresAt: j.SheetExpiresAt, DownloadedAt: j.SheetDownloadedAt}
	state.Available = j.Status == ImportDone && j.Created > 0 && j.SheetDownloadedAt == nil &&
		j.SheetExpiresAt != nil && now.Before(*j.SheetExpiresAt)
	return state, true
}
