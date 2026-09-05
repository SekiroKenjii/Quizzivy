package assignments

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrNotClosed    = errors.New("assignments: not closed")
	ErrBlankReason  = errors.New("assignments: reason is blank")
	ErrClosesInPast = errors.New("assignments: closes_at is not ahead")
)

// Reopen is G-09's "Gia hạn cho tất cả": a closed assignment gets a later
// closes_at and any early close is lifted, so every student with attempts
// left can go back in. Only a closed assignment qualifies, judged at the
// database's clock like the list, and the audit row is written from the
// UPDATE's own OLD/NEW so the values recorded are the values changed (§13.4).
func (s *Store) Reopen(ctx context.Context, req Request, closesAt time.Time, reason string, now time.Time) (Assignment, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return Assignment{}, ErrBlankReason
	}
	if !closesAt.After(now) {
		return Assignment{}, ErrClosesInPast
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Assignment{}, fmt.Errorf("assignments: begin reopen: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var reopened string
	err = tx.QueryRow(ctx, `
		WITH updated AS (
		  UPDATE app.assignments a
		     SET closes_at = $2, closed_at = NULL
		   WHERE a.id = $1::uuid AND `+derivedStatus+` = 'closed'
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
		req.ID, closesAt, req.ActorID, now, req.IP, req.UserAgent, reason).Scan(&reopened)
	if errors.Is(err, pgx.ErrNoRows) {
		return Assignment{}, s.whyNotReopened(ctx, tx, req.ID)
	}
	if err != nil {
		return Assignment{}, fmt.Errorf("assignments: reopen: %w", err)
	}

	saved, err := s.get(ctx, tx, req.ID)
	if err != nil {
		return Assignment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Assignment{}, fmt.Errorf("assignments: commit reopen: %w", err)
	}
	return saved, nil
}

func (s *Store) whyNotReopened(ctx context.Context, q querier, id string) error {
	var exists bool
	if err := q.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM app.assignments WHERE id = $1::uuid)`, id).Scan(&exists); err != nil {
		return fmt.Errorf("assignments: reopen check: %w", err)
	}
	if !exists {
		return ErrNotFound
	}
	return ErrNotClosed
}
