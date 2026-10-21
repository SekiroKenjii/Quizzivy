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
		SELECT x.id FROM app.assignments x WHERE x.created_by = $1::uuid ORDER BY x.id FOR NO KEY UPDATE
	), moved AS (
		UPDATE app.assignments x SET created_by = $2::uuid FROM locked WHERE x.id = locked.id RETURNING true AS live
	)
	SELECT count(*) FILTER (WHERE live)::int FROM moved`, from, to).Scan(&moved); err != nil {
		return 0, fmt.Errorf("assignments: transfer ownership: %w", err)
	}
	return moved, nil
}
