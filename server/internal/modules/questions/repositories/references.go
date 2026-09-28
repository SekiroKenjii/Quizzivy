package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/opt"

	"github.com/jackc/pgx/v5"
)

// DraftReferences lists the draft tests whose outline uses the question, by
// title. Any at all blocks deletion with a 409.
func DraftReferences(ctx context.Context, q db.Querier, questionID string) ([]domain.TestRef, error) {
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
func LockForDraftUse(ctx context.Context, q db.Querier, questionID string) error {
	var deleted bool
	err := q.QueryRow(ctx,
		`SELECT deleted_at IS NOT NULL FROM app.questions WHERE id = $1 AND context_group_id IS NULL FOR UPDATE`,
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

// NotOwnedBy returns, in id order, the questions among questionIDs that
// ownerID does not own. It takes no lock: callers run it after LockForDraftUse
// has locked every one of them in the same transaction, so the answer holds
// until they commit.
func NotOwnedBy(ctx context.Context, q db.Querier, ownerID string, questionIDs []string) ([]string, error) {
	rows, err := q.Query(ctx,
		`SELECT id::text FROM app.questions
		  WHERE id = ANY($1::uuid[]) AND owner_id IS DISTINCT FROM $2::uuid
		  ORDER BY id`, questionIDs, ownerID)
	if err != nil {
		return nil, fmt.Errorf("questions: owners: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func (s *Postgres) NotOwnedBy(ctx context.Context, tx pgx.Tx, ownerID string, questionIDs []string) ([]string, error) {
	return NotOwnedBy(ctx, tx, ownerID, questionIDs)
}

func (s *Postgres) questionUses(ctx context.Context, scope access.Scope, questionID string) ([]domain.TestRef, error) {
	rows, err := s.Query(ctx, `SELECT DISTINCT t.id::text, t.title
 FROM app.test_section_questions sq
 JOIN app.test_sections section ON section.id = sq.test_section_id
 JOIN app.tests t ON t.id = section.test_id
 WHERE sq.question_id = $1 AND t.deleted_at IS NULL AND ($2::boolean OR t.owner_id = $3::uuid)
 ORDER BY t.title, t.id::text`, questionID, scope.All, opt.String(scope.UserID))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[domain.TestRef])
}
