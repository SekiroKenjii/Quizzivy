package attempts

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Request is who is intervening and from where, for the audit row every
// intervention writes (§13.4).
type Request struct {
	ActorID   string
	IP        string
	UserAgent string
}

var (
	ErrBlankReason   = errors.New("attempts: reason is blank")
	ErrAttemptVoided = errors.New("attempts: attempt is voided")
)

// Extend moves a live attempt's deadline and records why, in one statement.
//
// The audit row is written by a data-modifying CTE fed from the UPDATE's own
// OLD/NEW, so there is no read-then-write between the value the teacher saw
// and the value that changed (§13.4). The deadline may pass `closes_at`: that
// is what an accommodation is for, and the constraint only requires it to
// exceed `started_at`.
func (s *Store) Extend(ctx context.Context, req Request, attemptID string, minutes int, reason string, now time.Time) (Attempt, error) {
	reason, err := cleanReason(reason)
	if err != nil {
		return Attempt{}, err
	}
	q := `
		WITH updated AS (
		  UPDATE app.attempts
		     SET deadline_at = deadline_at + make_interval(mins => $2)
		   WHERE id = $1::uuid AND status = 'in_progress'
		  RETURNING ` + attemptColumns + `, old.deadline_at AS prev_deadline
		), logged AS (
		  INSERT INTO app.audit_log
		         (actor_user_id, action, entity, entity_id, occurred_at, ip, user_agent, diff)
		  SELECT $3::uuid, 'attempt.extended', 'attempt', updated.id, $4, $5::inet, $6,
		         jsonb_build_object(
		           'deadline_at', jsonb_build_object('old', updated.prev_deadline, 'new', updated.deadline_at),
		           'minutes', $2, 'reason', $7::text)
		    FROM updated
		)
		SELECT ` + attemptColumns + ` FROM updated`
	out, err := scanAttempt(s.pool.QueryRow(ctx, q,
		attemptID, minutes, req.ActorID, now, optionalIP(req.IP), optional(req.UserAgent), reason))
	if errors.Is(err, pgx.ErrNoRows) {
		return Attempt{}, s.whyNotLive(ctx, attemptID)
	}
	if err != nil {
		return Attempt{}, fmt.Errorf("attempts: extend: %w", err)
	}
	return out.Attempt, nil
}

// Void marks an attempt void with its reason. Nothing is deleted: the answers
// and the timeline stay readable to the teacher (§6.4), and the row keeps its
// place in the attempt numbering.
func (s *Store) Void(ctx context.Context, req Request, attemptID, reason string, now time.Time) (Attempt, error) {
	return s.void(ctx, req, attemptID, reason, "attempt.voided", now)
}

// Reset is Void under another name: the voided attempt no longer counts
// against `max_attempts`, so the student may start `attempt_no + 1` (O-08).
func (s *Store) Reset(ctx context.Context, req Request, attemptID, reason string, now time.Time) (Attempt, error) {
	return s.void(ctx, req, attemptID, reason, "attempt.reset", now)
}

func (s *Store) void(ctx context.Context, req Request, attemptID, reason, action string, now time.Time) (Attempt, error) {
	reason, err := cleanReason(reason)
	if err != nil {
		return Attempt{}, err
	}
	q := `
		WITH updated AS (
		  UPDATE app.attempts
		     SET status = 'voided', void_reason = $2
		   WHERE id = $1::uuid AND status <> 'voided'
		  RETURNING ` + attemptColumns + `, old.status AS prev_status
		), logged AS (
		  INSERT INTO app.audit_log
		         (actor_user_id, action, entity, entity_id, occurred_at, ip, user_agent, diff)
		  SELECT $3::uuid, $7::text, 'attempt', updated.id, $4, $5::inet, $6,
		         jsonb_build_object(
		           'status', jsonb_build_object('old', updated.prev_status::text, 'new', 'voided'),
		           'void_reason', jsonb_build_object('old', NULL, 'new', $2::text))
		    FROM updated
		)
		SELECT ` + attemptColumns + ` FROM updated`
	out, err := scanAttempt(s.pool.QueryRow(ctx, q,
		attemptID, reason, req.ActorID, now, optionalIP(req.IP), optional(req.UserAgent), action))
	if errors.Is(err, pgx.ErrNoRows) {
		return Attempt{}, s.whyNotLive(ctx, attemptID)
	}
	if err != nil {
		return Attempt{}, fmt.Errorf("attempts: %s: %w", action, err)
	}
	return out.Attempt, nil
}

// whyNotLive turns "no row updated" into the reason: missing, or in a status
// the action does not apply to.
func (s *Store) whyNotLive(ctx context.Context, attemptID string) error {
	var status Status
	err := s.pool.QueryRow(ctx,
		`SELECT status FROM app.attempts WHERE id = $1::uuid`, attemptID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("attempts: read status: %w", err)
	}
	if status == Voided {
		return ErrAttemptVoided
	}
	return ErrAttemptClosed
}

// cleanReason rejects a reason that is only whitespace: the schema's minLength
// cannot tell "   " from a reason, and a blank one defeats the audit row.
func cleanReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", ErrBlankReason
	}
	return reason, nil
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// optionalIP is optional for the inet column, which rejects an empty string
// where a text column would store it.
func optionalIP(s string) *string { return optional(s) }

func (s *Service) Extend(ctx context.Context, req Request, attemptID string, minutes int, reason string) (Attempt, error) {
	return s.store.Extend(ctx, req, attemptID, minutes, reason, s.now())
}

func (s *Service) Void(ctx context.Context, req Request, attemptID, reason string) (Attempt, error) {
	return s.store.Void(ctx, req, attemptID, reason, s.now())
}

func (s *Service) Reset(ctx context.Context, req Request, attemptID, reason string) (Attempt, error) {
	return s.store.Reset(ctx, req, attemptID, reason, s.now())
}
