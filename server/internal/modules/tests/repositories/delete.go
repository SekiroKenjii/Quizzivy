package repositories

import (
	"context"
	"errors"
	"quizzivy/internal/modules/tests/domain"
	"time"

	"github.com/jackc/pgx/v5"
)

// Delete permanently removes an archived test only when no assignment or attempt references it.
func (s *Postgres) Delete(ctx context.Context, req domain.Request, now time.Time) error {
	tx, err := s.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status string
	err = tx.QueryRow(ctx, `SELECT status::text FROM app.tests WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, req.ID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != string(domain.Archived) {
		return domain.ErrNotArchived
	}
	if _, err := tx.Exec(ctx, `DELETE FROM app.test_versions WHERE test_id = $1`, req.ID); err != nil {
		return referenceError(err)
	}
	if err := clearTestGroups(ctx, tx, req.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM app.tests WHERE id = $1`, req.ID); err != nil {
		return referenceError(err)
	}
	if err := auditTestChange(ctx, tx, req, now, "test.deleted"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeleteVersion removes an unused snapshot while preserving the current default and assigned history.
func (s *Postgres) DeleteVersion(ctx context.Context, req domain.VersionRequest, now time.Time) error {
	tx, err := s.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var current int
	err = tx.QueryRow(ctx, `SELECT current_version FROM app.tests WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, req.ID).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if current == req.Version {
		return domain.ErrCurrentVersion
	}
	versionID, err := lockVersion(ctx, tx, req)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM app.test_versions WHERE id = $1`, versionID); err != nil {
		return referenceError(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE app.tests SET updated_at = now() WHERE id = $1`, req.ID); err != nil {
		return err
	}
	if err := auditVersionChange(ctx, tx, req, now, "test.version_deleted"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
