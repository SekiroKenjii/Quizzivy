package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/tests/domain"
)

// TransferOwnership implements domain.OwnershipTransfer.
func (s *Postgres) TransferOwnership(ctx context.Context, from, to string) (domain.Transferred, error) {
	var moved domain.Transferred
	if err := s.QueryRow(ctx, `
	WITH locked AS (
		SELECT x.id FROM app.tests x WHERE x.owner_id = $1::uuid ORDER BY x.id FOR NO KEY UPDATE
	), moved AS (
		UPDATE app.tests x SET owner_id = $2::uuid FROM locked WHERE x.id = locked.id RETURNING x.deleted_at IS NULL AS live
	)
	SELECT count(*) FILTER (WHERE live)::int FROM moved`, from, to).Scan(&moved.Tests); err != nil {
		return domain.Transferred{}, fmt.Errorf("tests: transfer ownership: %w", err)
	}
	if err := s.QueryRow(ctx, `
	WITH locked AS (
		SELECT x.id FROM app.question_groups x WHERE x.owner_id = $1::uuid ORDER BY x.id FOR NO KEY UPDATE
	), moved AS (
		UPDATE app.question_groups x SET owner_id = $2::uuid FROM locked WHERE x.id = locked.id RETURNING x.archived_at IS NULL AS live
	)
	SELECT count(*) FILTER (WHERE live)::int FROM moved`, from, to).Scan(&moved.Groups); err != nil {
		return domain.Transferred{}, fmt.Errorf("tests: transfer question group ownership: %w", err)
	}
	return moved, nil
}
