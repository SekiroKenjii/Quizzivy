package questions

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Querier is the subset of pgx satisfied by both a pool and a transaction.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// CountDraftReferences reports how many draft test outlines use the question.
// A non-zero count blocks deletion with a 409.
//
// Runs on the caller's querier so it can share the transaction that locked the
// question, and is served by test_section_questions_question_idx.
func CountDraftReferences(ctx context.Context, q Querier, questionID string) (int, error) {
	var n int
	err := q.QueryRow(ctx,
		`SELECT count(*) FROM app.test_section_questions WHERE question_id = $1`,
		questionID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("questions: count draft references: %w", err)
	}
	return n, nil
}

// LockForDraftUse takes the row lock that makes the delete check meaningful,
// and must be called before inserting into app.test_section_questions.
func LockForDraftUse(ctx context.Context, q Querier, questionID string) error {
	var deleted bool
	err := q.QueryRow(ctx,
		`SELECT deleted_at IS NOT NULL FROM app.questions WHERE id = $1 FOR UPDATE`,
		questionID).Scan(&deleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("questions: lock for draft use: %w", err)
	}
	if deleted {
		return ErrNotFound
	}
	return nil
}
