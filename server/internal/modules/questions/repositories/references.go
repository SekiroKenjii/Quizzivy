package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/questions/domain"

	"github.com/jackc/pgx/v5"
)

// Querier is the subset of pgx satisfied by both a pool and a transaction.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// DraftReferences lists the draft tests whose outline uses the question, by
// title. Any at all blocks deletion with a 409.
func DraftReferences(ctx context.Context, q Querier, questionID string) ([]domain.TestRef, error) {
	rows, err := q.Query(ctx, `
		SELECT DISTINCT t.id::text, t.title
		  FROM app.test_section_questions tsq
		  JOIN app.test_sections ts ON ts.id = tsq.test_section_id
		  JOIN app.tests t ON t.id = ts.test_id
		 WHERE tsq.question_id = $1
		 ORDER BY t.title, t.id::text`, questionID)
	if err != nil {
		return nil, fmt.Errorf("questions: draft references: %w", err)
	}
	defer rows.Close()
	var out []domain.TestRef
	for rows.Next() {
		var ref domain.TestRef
		if err := rows.Scan(&ref.ID, &ref.Title); err != nil {
			return nil, fmt.Errorf("questions: draft references: %w", err)
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

// LockForDraftUse takes the row lock that makes the delete check meaningful,
// and must be called before inserting into app.test_section_questions.
func LockForDraftUse(ctx context.Context, q Querier, questionID string) error {
	var deleted bool
	err := q.QueryRow(ctx,
		`SELECT deleted_at IS NOT NULL FROM app.questions WHERE id = $1 FOR UPDATE`,
		questionID).Scan(&deleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("questions: lock for draft use: %w", err)
	}
	if deleted {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Postgres) LockForDraftUse(ctx context.Context, tx pgx.Tx, questionID string) error {
	return LockForDraftUse(ctx, tx, questionID)
}
