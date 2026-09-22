package repositories

import (
	"context"
	"quizzivy/internal/modules/attempts/domain"
	"time"

	"github.com/jackc/pgx/v5"
)

func closeForFocusLimit(ctx context.Context, tx pgx.Tx, attemptID string, now time.Time) (bool, error) {
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
		return false, err
	}
	if _, err := gradeAndClose(ctx, tx, attemptID, versionID, domain.AutoSubmit, deadline, now); err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `UPDATE app.attempts SET flagged = true WHERE id = $1`, attemptID); err != nil {
		return false, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO app.attempt_events (attempt_id, session_id, kind, occurred_at, meta)
 SELECT id, session_id, 'auto_submit', $2, jsonb_build_object('reason', 'focus_loss_limit', 'focusLossCount', focus_loss_count)
 FROM app.attempts WHERE id = $1`, attemptID, now)
	return err == nil, err
}
