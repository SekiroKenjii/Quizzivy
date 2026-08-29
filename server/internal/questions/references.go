package questions

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Querier is the subset of pgx satisfied by both a pool and a transaction.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// CountDraftReferences reports how many draft test outlines use the question.
// A non-zero count blocks deletion with a 409.
//
// Always 0 until app.test_section_questions exists in T-2.7;
// TestDraftReferenceCheckIsWiredOnceTestTablesExist fails once it does.
func CountDraftReferences(ctx context.Context, q Querier, questionID string) (int, error) {
	_, _, _ = ctx, q, questionID
	return 0, nil
}
