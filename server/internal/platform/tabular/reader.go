package tabular

import (
	"errors"
	"fmt"
	"io"
	"iter"
	"strings"
	"unicode/utf8"
)

// Format names a file format ReadTable reads.
type Format string

const (
	// FormatCSV is UTF-8 text with comma- or semicolon-separated records.
	FormatCSV Format = "csv"
	// FormatXLSX is an Office Open XML workbook, without macros or a password.
	FormatXLSX Format = "xlsx"
)

var (
	// ErrFormat means the file is not a table of the format it was read as.
	ErrFormat = errors.New("tabular: the file is not a readable table")
	// ErrEmpty means the file holds no header row.
	ErrEmpty = errors.New("tabular: the file has no header row")
	// ErrNotUTF8 means a CSV file is in another encoding. Its message tells the
	// user to save it again as "CSV UTF-8".
	ErrNotUTF8 = errors.New(`tabular: the file is not UTF-8 text; save it as "CSV UTF-8" and upload it again`)
	// ErrEncrypted means the workbook is protected by a password.
	ErrEncrypted = errors.New("tabular: the workbook is password-protected")
	// ErrMacros means the workbook carries macros.
	ErrMacros = errors.New("tabular: the workbook contains macros")
	// ErrTooLarge means the file, or the workbook inside its archive, is
	// bigger than the limit.
	ErrTooLarge = errors.New("tabular: the file is larger than the limit")
	// ErrTooManyRows means the table holds more data rows than the limit.
	ErrTooManyRows = errors.New("tabular: the table has more rows than the limit")
	// ErrTooManyColumns means a row uses more columns than the limit.
	ErrTooManyColumns = errors.New("tabular: a row has more columns than the limit")
	// ErrCellTooLong means a cell holds more characters than the limit.
	ErrCellTooLong = errors.New("tabular: a cell is longer than the limit")
)

// Limits bounds what ReadTable accepts from one file. Every field is
// positive.
type Limits struct {
	// Bytes is the largest file, in bytes.
	Bytes int64
	// Rows is the most data rows, which do not include the header or blank
	// rows.
	Rows int
	// Columns is the most columns a row may use, not counting empty cells at
	// its end.
	Columns int
	// CellChars is the most characters a cell may hold.
	CellChars int
}

// DefaultLimits returns the bounds a user import is read under: a 5 MiB file
// of at most 2,000 rows, 20 columns and 1,000 characters a cell.
func DefaultLimits() Limits {
	return Limits{Bytes: 5 << 20, Rows: 2000, Columns: 20, CellChars: 1000}
}

// Row is one data row of a Table.
type Row struct {
	// Line is the line of the file the row starts on, which is the row number
	// a spreadsheet shows beside it, counting from 1 with the header.
	Line int
	// Cells holds the row's cells as text, at least as many as the header
	// has. A cell is returned as it was read: nothing is trimmed or run.
	Cells []string
}

type source interface {
	next() (cells []string, line int, err error)
	close() error
}

// Table is the header and the data rows of one file. It holds the file's
// resources until Close.
type Table struct {
	// Header holds the first non-blank row, each cell with its surrounding
	// space trimmed.
	Header []string

	src    source
	limits Limits
	rows   int
}

// ReadTable reads a CSV file or the first sheet of an XLSX workbook from r,
// which holds size bytes, and returns its header with a way to read the rows
// below it; the caller closes the Table. Nothing in the file is run: a formula
// cell yields the value the workbook cached for it, as text, and is never
// evaluated. A CSV file must be UTF-8, with an optional byte-order mark, and is
// split at commas or semicolons, whichever its first line holds more of. A
// workbook is inspected before any of it is unpacked, and refused when it holds
// more than 1,000 archive entries or expands past 32 MiB, when an XML part
// other than a worksheet is larger than 1 MiB (2 MiB for the shared strings),
// which is what a decoder expands many times over in memory, or when it carries
// macros or a password. Every refusal wraps one of this package's errors.
func ReadTable(r io.ReaderAt, size int64, format Format, limits Limits) (*Table, error) {
	if limits.Bytes < 1 || limits.Rows < 1 || limits.Columns < 1 || limits.CellChars < 1 {
		return nil, errors.New("tabular: every limit must be positive")
	}
	if size < 1 {
		return nil, ErrEmpty
	}
	if size > limits.Bytes {
		return nil, fmt.Errorf("%w: %d bytes, limit %d", ErrTooLarge, size, limits.Bytes)
	}
	var (
		src source
		err error
	)
	switch format {
	case FormatCSV:
		src, err = openCSV(r, size)
	case FormatXLSX:
		src, err = openXLSX(r, size)
	default:
		return nil, fmt.Errorf("tabular: unknown format %q", format)
	}
	if err != nil {
		return nil, err
	}
	table := &Table{src: src, limits: limits}
	if err := table.readHeader(); err != nil {
		return nil, errors.Join(err, src.close())
	}
	return table, nil
}

// Rows returns the data rows in file order. Blank rows are skipped and do not
// count against the limit. A row beyond a limit ends the sequence with an
// error, as does a file that turns out to be malformed. The sequence reads the
// file once: a second call continues where the first stopped.
func (t *Table) Rows() iter.Seq2[Row, error] {
	return func(yield func(Row, error) bool) {
		for {
			cells, line, err := t.src.next()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				yield(Row{}, err)
				return
			}
			cells = trimTrailingEmpty(cells)
			if len(cells) == 0 {
				continue
			}
			if t.rows == t.limits.Rows {
				yield(Row{}, fmt.Errorf("%w: row %d is beyond the %d allowed", ErrTooManyRows, line, t.limits.Rows))
				return
			}
			if err := t.limits.check(cells, line); err != nil {
				yield(Row{}, err)
				return
			}
			t.rows++
			if missing := len(t.Header) - len(cells); missing > 0 {
				cells = append(cells, make([]string, missing)...)
			}
			if !yield(Row{Line: line, Cells: cells}, nil) {
				return
			}
		}
	}
}

// Close releases what reading the file held: the workbook's temporary files,
// for one. It is safe to call more than once.
func (t *Table) Close() error {
	return t.src.close()
}

func (t *Table) readHeader() error {
	for {
		cells, line, err := t.src.next()
		if errors.Is(err, io.EOF) {
			return ErrEmpty
		}
		if err != nil {
			return err
		}
		cells = trimTrailingEmpty(cells)
		if len(cells) == 0 {
			continue
		}
		if err := t.limits.check(cells, line); err != nil {
			return err
		}
		for i, cell := range cells {
			cells[i] = strings.TrimSpace(cell)
		}
		t.Header = cells
		return nil
	}
}

func (l Limits) check(cells []string, line int) error {
	if len(cells) > l.Columns {
		return fmt.Errorf("%w: row %d uses %d, limit %d", ErrTooManyColumns, line, len(cells), l.Columns)
	}
	for i, cell := range cells {
		if utf8.RuneCountInString(cell) > l.CellChars {
			return fmt.Errorf("%w: row %d column %d, limit %d characters", ErrCellTooLong, line, i+1, l.CellChars)
		}
	}
	return nil
}

func trimTrailingEmpty(cells []string) []string {
	end := len(cells)
	for end > 0 && strings.TrimSpace(cells[end-1]) == "" {
		end--
	}
	return cells[:end]
}
