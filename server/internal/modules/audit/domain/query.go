package domain

import "time"

// The sizes the log is read in.
const (
	DefaultLimit = 10
	MaxLimit     = 100
	// MaxExportRows is the most entries one export holds.
	MaxExportRows = 100_000
	// ExportBatchSize is how many entries an export reads at a time.
	ExportBatchSize = 1000
	// StatementTimeout bounds each statement an export runs, and each search of the list.
	StatementTimeout = 30 * time.Second
)

// Filter is which entries a reader looks at. The empty Category is all of them.
type Filter struct {
	Category Category
	// Query is the text searched for, trimmed.
	Query string
	// Actions are the keys of the actions whose sentence Query matches.
	Actions []string
	Window  Window
	// Through, when above zero, leaves out the entries written after the one
	// with this id, so an export reads the log as it stood when it began.
	Through int64
}

// Cursor is where the next batch of an export starts: after the entry with this
// time and id, going back.
type Cursor struct {
	OccurredAt time.Time
	ID         int64
}

// ExportCount is what a count of an export's entries found.
type ExportCount struct {
	// Rows is how many entries there are, and MaxExportRows+1 when there are
	// more than MaxExportRows.
	Rows int
	// Through is the highest id among them.
	Through int64
}

// Export is a file the log is being written to, as it is audited.
type Export struct {
	ActorID   string
	IP        string
	UserAgent string
	At        time.Time
	Period    Range
	Format    string
	Category  Category
	Query     string
	Rows      int
}

// ExportPlan is an export that was begun: the entries it reads, how many, the
// zone its times are written in, and the moment it began.
type ExportPlan struct {
	Filter Filter
	Rows   int
	Zone   string
	At     time.Time
}
