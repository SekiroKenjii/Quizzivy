package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/opt"
	"quizzivy/internal/shared/schedule"
	"quizzivy/internal/shared/visibility"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Reopen is G-09's "Gia hạn cho tất cả": a closed assignment gets a later
// closes_at and any early close is lifted, so every student with attempts
// left can go back in. Only a closed assignment qualifies, judged at the
// database's clock like the list, and the audit row is written from the
// UPDATE's own OLD/NEW so the values recorded are the values changed (§13.4).
// Only an assignment the actor reaches qualifies; another teacher's answers
// ErrNotFound.
func (s *Postgres) Reopen(ctx context.Context, req domain.Request, closesAt time.Time, reason string, now time.Time) (domain.Assignment, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return domain.Assignment{}, domain.ErrBlankReason
	}
	if !closesAt.After(now) {
		return domain.Assignment{}, domain.ErrClosesInPast
	}

	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.Assignment{}, fmt.Errorf("assignments: begin reopen: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var reopened string
	err = tx.QueryRow(ctx, `
		WITH updated AS (
		  UPDATE app.assignments a
		     SET closes_at = $2, closed_at = NULL
		   WHERE a.id = $1::uuid AND `+schedule.DerivedStatus+` = 'closed'
		     AND ($8::boolean OR a.id IN `+visibility.AssignmentIDs(9)+`)
		  RETURNING a.id, old.closes_at AS prev_closes_at, old.closed_at AS prev_closed_at
		), logged AS (
		  INSERT INTO app.audit_log
		         (actor_user_id, action, entity, entity_id, occurred_at, ip, user_agent, diff)
		  SELECT $3::uuid, 'assignment.reopened', 'assignment', updated.id, $4, nullif($5, '')::inet, nullif($6, ''),
		         jsonb_build_object(
		           'closes_at', jsonb_build_object('old', updated.prev_closes_at, 'new', $2::timestamptz),
		           'closed_at', jsonb_build_object('old', updated.prev_closed_at, 'new', NULL),
		           'reason', $7::text)
		    FROM updated
		)
		SELECT id::text FROM updated`,
		req.ID, closesAt, req.ActorID, now, req.IP, req.UserAgent, reason, req.All, opt.String(req.ActorID)).Scan(&reopened)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Assignment{}, s.whyNotReopened(ctx, tx, req.Scope(), req.ID)
	}
	if err != nil {
		return domain.Assignment{}, fmt.Errorf("assignments: reopen: %w", err)
	}

	saved, err := s.get(ctx, tx, req.Scope(), req.ID, false)
	if err != nil {
		return domain.Assignment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Assignment{}, fmt.Errorf("assignments: commit reopen: %w", err)
	}
	return saved, nil
}

func (s *Postgres) whyNotReopened(ctx context.Context, q db.Querier, scope access.Scope, id string) error {
	var exists bool
	if err := q.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM app.assignments WHERE id = $1::uuid AND ($2::boolean OR id IN `+visibility.AssignmentIDs(3)+`))`,
		id, scope.All, opt.String(scope.UserID)).Scan(&exists); err != nil {
		return fmt.Errorf("assignments: reopen check: %w", err)
	}
	if !exists {
		return domain.ErrNotFound
	}
	return domain.ErrNotClosed
}
