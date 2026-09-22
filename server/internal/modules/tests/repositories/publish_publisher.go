package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Publish validates the draft, freezes it as a new version, bumps
// current_version, sets status published, and audits -- all in one transaction.
func (s *Postgres) Publish(ctx context.Context, req domain.PublishRequest, now time.Time, validate func(domain.DraftContent) error) (domain.PublishedVersion, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.PublishedVersion{}, fmt.Errorf("publish: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	current, err := lockTest(ctx, tx, req.TestID)
	if err != nil {
		return domain.PublishedVersion{}, err
	}

	draft, err := loadDraft(ctx, tx, req.TestID)
	if err != nil {
		return domain.PublishedVersion{}, err
	}
	if err := validate(draft); err != nil {
		return domain.PublishedVersion{}, err
	}

	total, count := domain.Publishing.Totals(draft)

	versionID, err := insertVersion(ctx, tx, req, current+1, total, now)
	if err != nil {
		return domain.PublishedVersion{}, err
	}
	if err := snapshot(ctx, tx, versionID, draft, s.media); err != nil {
		return domain.PublishedVersion{}, err
	}

	if _, err := tx.Exec(ctx,
		`UPDATE app.tests SET status = 'published', current_version = $2, last_published_version = $2 WHERE id = $1`,
		req.TestID, current+1); err != nil {
		return domain.PublishedVersion{}, fmt.Errorf("publish: bump current_version: %w", err)
	}

	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &req.ActorID,
		Action:      "test.published",
		Entity:      "test_version",
		EntityID:    &versionID,
		OccurredAt:  now,
		IP:          opt.String(req.IP),
		UserAgent:   opt.String(req.UserAgent),
	}); err != nil {
		return domain.PublishedVersion{}, err
	}

	published, err := readVersion(ctx, tx, versionID)
	if err != nil {
		return domain.PublishedVersion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.PublishedVersion{}, fmt.Errorf("publish: commit: %w", err)
	}
	published.QuestionCount = count
	return published, nil
}

func lockTest(ctx context.Context, tx pgx.Tx, testID string) (int, error) {
	var current int
	err := tx.QueryRow(ctx,
		`SELECT last_published_version FROM app.tests WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
		testID).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, domain.ErrDraftNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("publish: lock test: %w", err)
	}
	return current, nil
}

func insertVersion(ctx context.Context, tx pgx.Tx, req domain.PublishRequest, version int, total string, now time.Time) (string, error) {
	var id string
	if err := tx.QueryRow(ctx,
		`INSERT INTO app.test_versions (test_id, version, total_points, published_at, published_by)
		 VALUES ($1, $2, $3::numeric, $4, $5) RETURNING id::text`,
		req.TestID, version, total, now, req.ActorID).Scan(&id); err != nil {
		return "", fmt.Errorf("publish: insert version: %w", err)
	}
	return id, nil
}

func readVersion(ctx context.Context, tx pgx.Tx, versionID string) (domain.PublishedVersion, error) {
	var v domain.PublishedVersion
	err := tx.QueryRow(ctx,
		`SELECT tv.id::text, tv.version, tv.total_points::text, tv.published_at, u.full_name
		   FROM app.test_versions tv
		   JOIN app.users u ON u.id = tv.published_by
		  WHERE tv.id = $1`, versionID).Scan(
		&v.ID, &v.Version, &v.TotalPoints, &v.PublishedAt, &v.PublishedBy)
	if err != nil {
		return domain.PublishedVersion{}, fmt.Errorf("publish: read version: %w", err)
	}
	return v, nil
}
