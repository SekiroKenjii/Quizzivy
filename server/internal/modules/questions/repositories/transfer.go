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
		SELECT x.id FROM app.questions x WHERE x.owner_id = $1::uuid ORDER BY x.id FOR NO KEY UPDATE
	), moved AS (
		UPDATE app.questions x SET owner_id = $2::uuid FROM locked WHERE x.id = locked.id RETURNING x.deleted_at IS NULL AS live
	)
	SELECT count(*) FILTER (WHERE live)::int FROM moved`, from, to).Scan(&moved); err != nil {
		return 0, fmt.Errorf("questions: transfer ownership: %w", err)
	}
	return moved, nil
}
