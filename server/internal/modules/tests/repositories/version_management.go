package repositories

import (
	"context"
	"errors"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// SetCurrentVersion changes the default for future assignments without modifying snapshots.
func (s *Postgres) SetCurrentVersion(ctx context.Context, req domain.VersionRequest, now time.Time) (domain.Test, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.Test{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := checkVersion(ctx, tx, req.ID, req.ExpectedUpdatedAt); err != nil {
		return domain.Test{}, err
	}
	if _, err := lockVersion(ctx, tx, req); err != nil {
		return domain.Test{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE app.tests SET current_version = $2 WHERE id = $1`, req.ID, req.Version); err != nil {
		return domain.Test{}, err
	}
	return s.finishVersionChange(ctx, tx, req, now, "test.current_version_changed")
}

// CreateDraftFromVersion restores editable copies of the snapshot into the draft.
func (s *Postgres) CreateDraftFromVersion(ctx context.Context, req domain.VersionRequest, now time.Time) (domain.Test, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.Test{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := checkVersion(ctx, tx, req.ID, req.ExpectedUpdatedAt); err != nil {
		return domain.Test{}, err
	}
	versionID, err := lockVersion(ctx, tx, req)
	if err != nil {
		return domain.Test{}, err
	}
	if err := clearTestGroups(ctx, tx, req.ID); err != nil {
		return domain.Test{}, err
	}
	if err := s.lockVersionAssets(ctx, tx, versionID); err != nil {
		return domain.Test{}, err
	}
	copies, err := s.copyVersionDraft(ctx, tx, versionID, req.ActorID)
	if err != nil {
		return domain.Test{}, err
	}
	sections := make([]domain.SectionInput, len(copies))
	for i, section := range copies {
		sections[i] = section.Input
	}
	if err := s.lockQuestions(ctx, tx, sections); err != nil {
		return domain.Test{}, err
	}
	if err := replaceOutline(ctx, tx, req.ID, sections); err != nil {
		return domain.Test{}, err
	}
	if err := s.restoreSnapshotGroups(ctx, tx, req, now, copies); err != nil {
		return domain.Test{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE app.tests SET updated_at = now() WHERE id = $1`, req.ID); err != nil {
		return domain.Test{}, err
	}
	return s.finishVersionChange(ctx, tx, req, now, "test.draft_restored")
}

func lockVersion(ctx context.Context, tx pgx.Tx, req domain.VersionRequest) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `SELECT id::text FROM app.test_versions WHERE test_id = $1 AND version = $2 FOR UPDATE`, req.ID, req.Version).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	return id, err
}

func (s *Postgres) finishVersionChange(ctx context.Context, tx pgx.Tx, req domain.VersionRequest, now time.Time, action string) (domain.Test, error) {
	if err := auditVersionChange(ctx, tx, req, now, action); err != nil {
		return domain.Test{}, err
	}
	saved, err := s.get(ctx, tx, req.ID)
	if err != nil {
		return domain.Test{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Test{}, err
	}
	return saved, nil
}

func auditTestChange(ctx context.Context, tx pgx.Tx, req domain.Request, now time.Time, action string) error {
	return audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &req.ActorID, Action: action, Entity: entityTest, EntityID: &req.ID,
		OccurredAt: now, IP: opt.String(req.IP), UserAgent: opt.String(req.UserAgent),
	})
}

func referenceError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23001") {
		return domain.ErrReferenced
	}
	return err
}

func auditVersionChange(ctx context.Context, tx pgx.Tx, req domain.VersionRequest, now time.Time, action string) error {
	return audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &req.ActorID, Action: action, Entity: entityTest, EntityID: &req.ID,
		OccurredAt: now, IP: opt.String(req.IP), UserAgent: opt.String(req.UserAgent),
		Diff: []byte(`{"version":` + strconv.Itoa(req.Version) + `}`),
	})
}
