package repositories

import (
	"context"
	"quizzivy/internal/modules/attempts/domain"
	"time"

	"github.com/jackc/pgx/v5"
)

type focusClose struct {
	closed  bool
	flagged bool
}

func closeForFocusLimit(ctx context.Context, tx pgx.Tx, attemptID string, now time.Time) (focusClose, error) {
	var exceeded bool
	var versionID string
	var deadline time.Time
	err := tx.QueryRow(ctx, `SELECT at.test_version_id::text, at.deadline_at,
  at.status = 'in_progress' AND a.integrity_on_limit_exceeded = 'auto_submit'
  AND a.integrity_max_focus_loss <> 0
  AND at.focus_loss_count > greatest(0, a.integrity_max_focus_loss)
 FROM app.attempts at JOIN app.assignments a ON a.id = at.assignment_id
 WHERE at.id = $1`, attemptID).Scan(&versionID, &deadline, &exceeded)
	if err != nil || !exceeded {
		return focusClose{}, err
	}
	if _, err := gradeAndClose(ctx, tx, attemptID, versionID, domain.AutoSubmit, deadline, now); err != nil {
		return focusClose{}, err
	}
	var wasFlagged bool
	if err := tx.QueryRow(ctx, `UPDATE app.attempts SET flagged = true WHERE id = $1 RETURNING old.flagged`, attemptID).Scan(&wasFlagged); err != nil {
		return focusClose{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO app.attempt_events (attempt_id, session_id, kind, occurred_at, meta)
 SELECT id, session_id, 'auto_submit', $2, jsonb_build_object('reason', 'focus_loss_limit', 'focusLossCount', focus_loss_count)
 FROM app.attempts WHERE id = $1`, attemptID, now)
	if err != nil {
		return focusClose{}, err
	}
	return focusClose{closed: true, flagged: !wasFlagged}, nil
}
