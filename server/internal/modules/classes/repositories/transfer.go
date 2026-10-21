package repositories

import (
	"context"
	"fmt"
)

// TransferOwnership implements domain.OwnershipTransfer.
func (s *Postgres) TransferOwnership(ctx context.Context, from, to string) (int, error) {
	var moved int
	if err := s.QueryRow(ctx, `
	WITH locked AS (
		SELECT x.id FROM app.classes x WHERE x.teacher_id = $1::uuid ORDER BY x.id FOR NO KEY UPDATE
	), moved AS (
		UPDATE app.classes x SET teacher_id = $2::uuid FROM locked WHERE x.id = locked.id RETURNING true AS live
	)
	SELECT count(*) FILTER (WHERE live)::int FROM moved`, from, to).Scan(&moved); err != nil {
		return 0, fmt.Errorf("classes: transfer ownership: %w", err)
	}
	return moved, nil
}
