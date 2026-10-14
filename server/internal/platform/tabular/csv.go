// Package tabular is where a table crosses the product's boundary. It writes
// the tables people download so that a spreadsheet opens them as the data they
// are and never runs a cell as a formula, and it reads the CSV and XLSX files
// they upload within limits that bound what a hostile file can cost.
package tabular

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

const byteOrderMark = "\xEF\xBB\xBF"

var formulaTriggers = map[rune]bool{
	'=': true, '+': true, '-': true, '@': true, '\t': true, '\r': true,
	'＝': true, '＋': true, '－': true, '＠': true,
}

var leadingSpaces = map[rune]bool{' ': true, 0x00A0: true, 0x3000: true}

// EscapeCell returns value ready to be a cell: a value a spreadsheet would run
// as a formula gets a leading apostrophe, which the spreadsheet shows as text
// and does not run. That is a value whose first character, after any leading
// spaces, is "=", "+", "-", "@", a tab or a carriage return, or the full-width
// form of the first four. Any other value is returned as it is.
func EscapeCell(value string) string {
	for _, r := range value {
		if leadingSpaces[r] {
			continue
		}
		if formulaTriggers[r] {
			return "'" + value
		}
		break
	}
	return value
}

// CSV writes records as comma-separated values for a spreadsheet: a UTF-8
// byte-order mark first, then records ended by CRLF, every cell passed through
// EscapeCell.
type CSV struct {
	out *csv.Writer
}

// NewCSV starts a file on w and writes its byte-order mark.
func NewCSV(w io.Writer) (*CSV, error) {
	if _, err := io.WriteString(w, byteOrderMark); err != nil {
		return nil, fmt.Errorf("tabular: write byte-order mark: %w", err)
	}
	out := csv.NewWriter(w)
	out.UseCRLF = true
	return &CSV{out: out}, nil
}

// Row writes one record.
func (c *CSV) Row(cells ...string) error {
	escaped := make([]string, len(cells))
	for i, cell := range cells {
		escaped[i] = EscapeCell(cell)
	}
	if err := c.out.Write(escaped); err != nil {
		return fmt.Errorf("tabular: write row: %w", err)
	}
	return nil
}

// Flush writes what is buffered and reports the first error any write met.
func (c *CSV) Flush() error {
	c.out.Flush()
	if err := c.out.Error(); err != nil {
		return fmt.Errorf("tabular: flush: %w", err)
	}
	return nil
}

type csvSource struct {
	in *csv.Reader
}

func openCSV(src io.ReaderAt, size int64) (source, error) {
	data, err := io.ReadAll(io.NewSectionReader(src, 0, size))
	if err != nil {
		return nil, fmt.Errorf("tabular: read file: %w", err)
	}
	data = bytes.TrimPrefix(data, []byte(byteOrderMark))
	if !utf8.Valid(data) {
		return nil, ErrNotUTF8
	}
	comma := delimiter(data)
	if err := boundFields(data, byte(comma)); err != nil {
		return nil, err
	}
	in := csv.NewReader(bytes.NewReader(data))
	in.Comma = comma
	in.FieldsPerRecord = -1
	return &csvSource{in: in}, nil
}

func (s *csvSource) next() ([]string, int, error) {
	record, err := s.in.Read()
	if errors.Is(err, io.EOF) {
		return nil, 0, io.EOF
	}
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %w", ErrFormat, err)
	}
	line, _ := s.in.FieldPos(0)
	return record, line, nil
}

func (s *csvSource) close() error {
	return nil
}

func delimiter(data []byte) rune {
	var commas, semicolons int
	quoted := false
scan:
	for _, b := range bytes.TrimLeft(data, "\r\n") {
		switch {
		case b == '"':
			quoted = !quoted
		case quoted:
		case b == '\n':
			break scan
		case b == ',':
			commas++
		case b == ';':
			semicolons++
		}
	}
	if semicolons > commas {
		return ';'
	}
	return ','
}

const (
	fieldStart = iota
	inUnquoted
	inQuoted
	afterQuote
)

type fieldScanner struct {
	comma      byte
	state      int
	fields     int
	line       int
	recordLine int
}

func boundFields(data []byte, comma byte) error {
	scan := fieldScanner{comma: comma, fields: 1, line: 1, recordLine: 1}
	for _, c := range data {
		scan.step(c)
		if scan.fields > maxGridColumns {
			return fmt.Errorf("%w: row %d has more than %d fields", ErrTooManyColumns, scan.recordLine, maxGridColumns)
		}
	}
	return nil
}

func (s *fieldScanner) step(c byte) {
	if s.state == inQuoted {
		s.inQuotes(c)
		return
	}
	switch c {
	case s.comma:
		s.fields++
		s.state = fieldStart
	case '\n':
		s.line++
		s.recordLine = s.line
		s.fields = 1
		s.state = fieldStart
	case '"':
		s.state = s.afterOpeningQuote()
	default:
		s.state = inUnquoted
	}
}

func (s *fieldScanner) afterOpeningQuote() int {
	if s.state == inUnquoted {
		return inUnquoted
	}
	return inQuoted
}

func (s *fieldScanner) inQuotes(c byte) {
	switch c {
	case '"':
		s.state = afterQuote
	case '\n':
		s.line++
	}
}
