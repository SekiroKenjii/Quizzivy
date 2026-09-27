// Package repositories reads maintenance windows from app.maintenance_windows, and guards an attempt start against them inside the caller's transaction.
package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/availability/domain"
	"quizzivy/internal/platform/db"
)

// WindowLockKey is the maintenance-windows key in the 73819 advisory-lock
// namespace (docs/plan/20-data-model.md §28). The operator's window-schedule
// takes it exclusively; an attempt start takes it shared.
const WindowLockKey = 40

// Postgres reads windows from the database.
type Postgres struct{ db.Repository }

// NewPostgres builds the repository on the database context.
func NewPostgres(dbx db.Context) *Postgres { return &Postgres{Repository: db.NewRepository(dbx)} }

var _ domain.Repository = (*Postgres)(nil)

// Next returns the earliest window that was not cancelled and ends after now.
func (s *Postgres) Next(ctx context.Context, now time.Time) (*domain.Window, error) {
	return first(s.QueryRow(ctx, `
		SELECT starts_at, ends_at FROM app.maintenance_windows
		 WHERE cancelled_at IS NULL AND ends_at > $1
		 ORDER BY starts_at LIMIT 1`, now))
}

// GuardAttemptStart returns the first window that was not cancelled and that
// an attempt running from now until deadline would overlap, or nil. It first
// takes the maintenance-windows advisory lock shared, so q must be the
// transaction that goes on to insert the attempt: a window scheduled while
// that transaction is open waits for it, and then extends the new attempt.
func GuardAttemptStart(ctx context.Context, q db.Querier, now, deadline time.Time) (*domain.Window, error) {
	if _, err := q.Exec(ctx, `SELECT pg_advisory_xact_lock_shared(73819, $1)`, WindowLockKey); err != nil {
		return nil, fmt.Errorf("availability: lock maintenance windows: %w", err)
	}
	return first(q.QueryRow(ctx, `
		SELECT starts_at, ends_at FROM app.maintenance_windows
		 WHERE cancelled_at IS NULL AND tstzrange(starts_at, ends_at) && tstzrange($1, $2)
		 ORDER BY starts_at LIMIT 1`, now, deadline))
}

func first(row pgx.Row) (*domain.Window, error) {
	var w domain.Window
	err := row.Scan(&w.StartsAt, &w.EndsAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("availability: read window: %w", err)
	}
	return &w, nil
}
