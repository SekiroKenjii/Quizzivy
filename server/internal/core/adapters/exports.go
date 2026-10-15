package adapters

import (
	"io"

	exportports "quizzivy/internal/modules/exports/application/ports"
	"quizzivy/internal/platform/tabular"
)

// ExportTable starts the CSV of one export file on w through tabular, which
// writes its byte-order mark and escapes every cell a spreadsheet would run as
// a formula. It is the exports module's NewTable.
func ExportTable(w io.Writer) (exportports.Table, error) {
	table, err := tabular.NewCSV(w)
	if err != nil {
		return nil, err
	}
	return table, nil
}
