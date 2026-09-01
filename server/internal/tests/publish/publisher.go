package publish

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/audit"
)

// Publisher turns a draft into a new immutable version.
type Publisher struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func NewPublisher(pool *pgxpool.Pool) *Publisher {
	return &Publisher{pool: pool, now: time.Now}
}

// Request is one publish.
type Request struct {
	TestID    string
	ActorID   string
	IP        string
	UserAgent string
}

// Publish validates the draft, freezes it as a new version, bumps
// current_version, sets status published, and audits -- all in one transaction.
//
// Republishing an unchanged test still creates a version. Versions are an
// append-only history of what was published and when, not a diff: an assignment
// names a version, so "nothing changed" still needs a row to point at.
func (p *Publisher) Publish(ctx context.Context, req Request) (Version, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return Version{}, fmt.Errorf("publish: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	current, err := lockTest(ctx, tx, req.TestID)
	if err != nil {
		return Version{}, err
	}

	draft, err := loadDraft(ctx, tx, req.TestID)
	if err != nil {
		return Version{}, err
	}
	if err := Validate(draft); err != nil {
		return Version{}, err
	}

	total, count := totals(draft)
	now := p.now()

	versionID, err := insertVersion(ctx, tx, req, current+1, total, now)
	if err != nil {
		return Version{}, err
	}
	if err := snapshot(ctx, tx, versionID, draft); err != nil {
		return Version{}, err
	}

	if _, err := tx.Exec(ctx,
		`UPDATE app.tests SET status = 'published', current_version = $2 WHERE id = $1`,
		req.TestID, current+1); err != nil {
		return Version{}, fmt.Errorf("publish: bump current_version: %w", err)
	}

	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &req.ActorID,
		Action:      "test.published",
		Entity:      "test_version",
		EntityID:    &versionID,
		OccurredAt:  now,
		IP:          optional(req.IP),
		UserAgent:   optional(req.UserAgent),
	}); err != nil {
		return Version{}, err
	}

	published, err := readVersion(ctx, tx, versionID)
	if err != nil {
		return Version{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Version{}, fmt.Errorf("publish: commit: %w", err)
	}
	published.QuestionCount = count
	return published, nil
}

// lockTest takes the row lock and returns the current version number, so two
// concurrent publishes of the same test cannot both claim the same number.
func lockTest(ctx context.Context, tx pgx.Tx, testID string) (int, error) {
	var current int
	err := tx.QueryRow(ctx,
		`SELECT current_version FROM app.tests WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
		testID).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("publish: lock test: %w", err)
	}
	return current, nil
}

// totals sums the points the version is scored out of, frozen so the
// denominator on an old attempt cannot drift.
func totals(d Draft) (string, int) {
	var total float64
	count := 0
	for _, section := range d.Sections {
		for _, q := range section.Questions {
			total += parsePoints(q.Points)
			count++
		}
	}
	return formatPoints(total), count
}

func insertVersion(ctx context.Context, tx pgx.Tx, req Request, version int, total string, now time.Time) (string, error) {
	var id string
	if err := tx.QueryRow(ctx,
		`INSERT INTO app.test_versions (test_id, version, total_points, published_at, published_by)
		 VALUES ($1, $2, $3::numeric, $4, $5) RETURNING id::text`,
		req.TestID, version, total, now, req.ActorID).Scan(&id); err != nil {
		return "", fmt.Errorf("publish: insert version: %w", err)
	}
	return id, nil
}

func readVersion(ctx context.Context, tx pgx.Tx, versionID string) (Version, error) {
	var v Version
	err := tx.QueryRow(ctx,
		`SELECT tv.id::text, tv.version, tv.total_points::text, tv.published_at, u.full_name
		   FROM app.test_versions tv
		   JOIN app.users u ON u.id = tv.published_by
		  WHERE tv.id = $1`, versionID).Scan(
		&v.ID, &v.Version, &v.TotalPoints, &v.PublishedAt, &v.PublishedBy)
	if err != nil {
		return Version{}, fmt.Errorf("publish: read version: %w", err)
	}
	return v, nil
}

func optional(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
