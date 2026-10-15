package repositories

import (
	"context"
	"fmt"
	"time"
)

// Prune implements domain.UserImportRetention. It skips a job that is locked,
// so it never waits on a download in flight and cannot form a cycle with one.
func (u *UserImports) Prune(ctx context.Context, before time.Time, limit int) (int, error) {
	tag, err := u.Exec(ctx, `
		DELETE FROM app.user_import_jobs j
		 USING (SELECT id FROM app.user_import_jobs
		         WHERE completed_at < $1
		         ORDER BY completed_at, id
		         LIMIT $2
		         FOR UPDATE SKIP LOCKED) old
		 WHERE j.id = old.id`, before, limit)
	if err != nil {
		return 0, fmt.Errorf("user imports: prune finished jobs: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
