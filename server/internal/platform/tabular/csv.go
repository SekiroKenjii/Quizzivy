// Package tabular writes the tables teachers download, so that a spreadsheet
// opens them as the data they are and never runs a cell as a formula.
package tabular

import (
	"encoding/csv"
	"fmt"
	"io"
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
