package adapters

import (
	"bytes"
	"context"
	"time"

	"quizzivy/internal/modules/identity/application/model"
	identitydomain "quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/email"
	"quizzivy/internal/platform/tabular"
)

// UserImportTables reads a user-import file with the tabular package, under
// its default limits: 5 MiB, 2,000 rows, 20 columns and 1,000 characters a
// cell. It answers the package's own errors, which the transport words.
type UserImportTables struct{}

// Read implements the identity module's TableReader port.
func (UserImportTables) Read(file []byte, format identitydomain.ImportFormat) (identitydomain.ImportSource, error) {
	table, err := tabular.ReadTable(bytes.NewReader(file), int64(len(file)), tabular.Format(format), tabular.DefaultLimits())
	if err != nil {
		return identitydomain.ImportSource{}, err
	}
	defer func() { _ = table.Close() }()
	source := identitydomain.ImportSource{Header: table.Header}
	for row, err := range table.Rows() {
		if err != nil {
			return identitydomain.ImportSource{}, err
		}
		source.Rows = append(source.Rows, identitydomain.ImportCells{Line: row.Line, Cells: row.Cells})
	}
	return source, nil
}

// UserImportBudget answers the identity module's EmailBudget port from the email outbox.
type UserImportBudget struct {
	Budget email.Budget
}

// Remaining implements the identity module's EmailBudget port.
func (b UserImportBudget) Remaining(ctx context.Context, now time.Time) (model.EmailAllowance, error) {
	remaining, limit, err := b.Budget.Remaining(ctx, now)
	return model.EmailAllowance{Remaining: remaining, Limit: limit}, err
}
