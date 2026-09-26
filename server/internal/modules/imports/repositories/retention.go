package repositories

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/audit"
)

// CloseIdle cancels up to limit imports that wait on a teacher and that
// nothing has touched since before: uploads and draft saves touch the import
// row. It marks each as closed_idle, audits it, and returns the closed IDs.
func (s *Postgres) CloseIdle(ctx context.Context, before time.Time, limit int) ([]string, error) {
	var ids []string
	err := s.InTx(ctx, "close idle imports", func(tx pgx.Tx) error {
		var err error
		ids, err = db.QueryMany(ctx, tx, `WITH idle AS (
 SELECT id FROM app.word_imports
 WHERE files_removed_at IS NULL AND status IN ('awaiting_sources','failed','needs_review') AND updated_at<$1
 ORDER BY updated_at,id LIMIT $2 FOR UPDATE SKIP LOCKED)
 UPDATE app.word_imports w SET status='cancelled',closed_idle=true,revision=w.revision+1 FROM idle
 WHERE w.id=idle.id AND w.status IN ('awaiting_sources','failed','needs_review') AND w.updated_at<$1 RETURNING w.id::text`,
			[]any{before, limit}, scanID)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if err := auditRetention(ctx, tx, id, "import.closed_idle"); err != nil {
				return err
			}
		}
		return nil
	})
	return ids, err
}

// ExpiredFiles lists up to limit imports whose files are due for removal,
// ordered by (updated_at, id) after the given cursor: committed before
// committedBefore, cancelled before cancelledBefore, or closed as idle.
func (s *Postgres) ExpiredFiles(ctx context.Context, committedBefore, cancelledBefore time.Time, after domain.Cursor, limit int) ([]domain.Cursor, error) {
	return db.QueryMany(ctx, s.Conn(), `SELECT i.id::text,i.updated_at FROM app.word_imports i LEFT JOIN app.word_import_commits c ON c.import_id=i.id
 WHERE i.files_removed_at IS NULL AND i.status IN ('committed','cancelled') AND (i.updated_at,i.id)>($4,$5::uuid)
 AND ((i.status='committed' AND c.committed_at<$1) OR (i.status='cancelled' AND (i.closed_idle OR i.updated_at<$2)))
 ORDER BY i.updated_at,i.id LIMIT $3`, []any{committedBefore, cancelledBefore, limit, after.UpdatedAt, cursorID(after)}, func(row pgx.Rows) (domain.Cursor, error) {
		var v domain.Cursor
		err := row.Scan(&v.ID, &v.UpdatedAt)
		return v, err
	})
}

// FilesOf lists every storage key an import owns: its sources, pending ones
// included, and its artifacts.
func (s *Postgres) FilesOf(ctx context.Context, importID string) ([]string, error) {
	return db.QueryMany(ctx, s.Conn(), `SELECT storage_key FROM app.word_import_sources WHERE import_id=$1
 UNION ALL SELECT storage_key FROM app.word_import_artifacts WHERE import_id=$1`, []any{importID}, scanID)
}

// FilesRemoved deletes a terminal import's review draft and marks its files
// removed. Call it only once its objects are gone; a second call is a no-op.
func (s *Postgres) FilesRemoved(ctx context.Context, importID string) error {
	return s.InTx(ctx, "remove import files", func(tx pgx.Tx) error {
		var status string
		var removed *time.Time
		if err := tx.QueryRow(ctx, `SELECT status,files_removed_at FROM app.word_imports WHERE id=$1 FOR UPDATE`, importID).Scan(&status, &removed); err != nil {
			return err
		}
		if removed != nil {
			return nil
		}
		if status != "committed" && status != "cancelled" {
			return domain.ErrConflict
		}
		if _, err := tx.Exec(ctx, `DELETE FROM app.word_import_drafts WHERE import_id=$1`, importID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE app.word_imports SET files_removed_at=clock_timestamp(),revision=revision+1 WHERE id=$1`, importID); err != nil {
			return err
		}
		return auditRetention(ctx, tx, importID, "import.files_removed")
	})
}

func scanID(row pgx.Rows) (string, error) {
	var id string
	err := row.Scan(&id)
	return id, err
}

func cursorID(c domain.Cursor) string {
	if c.ID == "" {
		return "00000000-0000-0000-0000-000000000000"
	}
	return c.ID
}

func touchImport(ctx context.Context, tx pgx.Tx, id string) error {
	_, err := tx.Exec(ctx, `UPDATE app.word_imports SET updated_at=now() WHERE id=$1`, id)
	return err
}

func auditRetention(ctx context.Context, tx pgx.Tx, id, action string) error {
	return audit.Write(ctx, tx, audit.Entry{Entity: "word_import", EntityID: &id, Action: action, OccurredAt: time.Now()})
}
