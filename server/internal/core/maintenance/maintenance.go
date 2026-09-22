// Package maintenance implements explicit privileged retention operations outside the API process.
package maintenance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"quizzivy/internal/platform/db"
)

// RetentionReport describes one bounded batch using the database's UTC calendar cutoff.
type RetentionReport struct {
	Cutoff  time.Time `json:"cutoff"`
	Rows    int64     `json:"rows"`
	Applied bool      `json:"applied"`
}

const retentionCandidates = `SELECT e.id FROM app.attempt_events e
 JOIN app.attempts at ON at.id=e.attempt_id
 JOIN app.assignments a ON a.id=at.assignment_id
 WHERE e.received_at < $1 AND coalesce(a.closed_at,a.closes_at) < $1
 ORDER BY e.received_at,e.id LIMIT $2`

// RetainEvents counts or deletes a bounded batch after both receipt and assignment close are thirteen months old.
// Apply requires the owner role; no application-role privileges are changed.
func RetainEvents(ctx context.Context, conn db.Querier, apply bool, batch int) (RetentionReport, error) {
	var out RetentionReport
	if batch < 1 || batch > 10000 {
		return out, errors.New("batch must be between 1 and 10000")
	}
	err := conn.QueryRow(ctx, `SELECT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC' - interval '13 months') AT TIME ZONE 'UTC'`).Scan(&out.Cutoff)
	if err != nil {
		return out, err
	}
	if !apply {
		err = conn.QueryRow(ctx, `SELECT count(id) FROM (`+retentionCandidates+`) AS candidates`, out.Cutoff, batch).Scan(&out.Rows)
		return out, err
	}
	err = conn.QueryRow(ctx, `
		WITH candidates AS (`+retentionCandidates+` FOR UPDATE OF e SKIP LOCKED
		), removed AS (
		  DELETE FROM app.attempt_events e USING candidates c WHERE e.id = c.id RETURNING e.id
		), logged AS (
		  INSERT INTO app.audit_log (action, entity, diff)
		  SELECT 'integrity.retained', 'attempt_events',
		    jsonb_build_object('cutoff', $1::timestamptz, 'deletedRows', count(id), 'databaseRole', current_user)
		  FROM removed HAVING count(id) > 0 RETURNING id
		)
		SELECT count(id) FROM removed`, out.Cutoff, batch).Scan(&out.Rows)
	out.Applied = err == nil
	return out, err
}

// AnonymizationReport identifies the student without exposing their previous identity.
type AnonymizationReport struct {
	StudentID string `json:"studentId"`
	Applied   bool   `json:"applied"`
}

// AnonymizeStudent removes structured identity and login credentials on an explicit request.
// Attempts, memberships and the append-only audit trail remain available for historical review.
func AnonymizeStudent(ctx context.Context, conn db.Conn, studentID string, apply bool) (AnonymizationReport, error) {
	out := AnonymizationReport{StudentID: studentID}
	if _, err := uuid.Parse(studentID); err != nil {
		return out, errors.New("student ID must be a UUID")
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var role string
	if err := tx.QueryRow(ctx, `SELECT role::text FROM app.users WHERE id = $1 FOR UPDATE`, studentID).Scan(&role); err != nil {
		return out, fmt.Errorf("find student: %w", err)
	}
	if role != "student" {
		return out, errors.New("only student accounts may be anonymized")
	}
	var active bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.attempts WHERE student_id = $1 AND status = 'in_progress')`, studentID).Scan(&active); err != nil {
		return out, err
	}
	if active {
		return out, errors.New("finish or void the student's active attempts before anonymizing")
	}
	if !apply {
		return out, nil
	}
	for _, statement := range []string{
		`DELETE FROM app.user_identities WHERE user_id = $1`,
		`DELETE FROM app.refresh_tokens WHERE user_id = $1`,
	} {
		if _, err := tx.Exec(ctx, statement, studentID); err != nil {
			return out, err
		}
	}
	_, err = tx.Exec(ctx, `
		WITH changed AS (
		  UPDATE app.users SET email = 'anonymous-' || id::text || '@anonymous.invalid',
		    full_name = 'Học viên đã ẩn danh', password_hash = NULL,
		    must_change_password = false, disabled_at = coalesce(disabled_at, now())
		  WHERE id = $1 AND email <> 'anonymous-' || id::text || '@anonymous.invalid'
		  RETURNING id
		)
		INSERT INTO app.audit_log (action, entity, entity_id, diff)
		SELECT 'student.anonymized', 'users', id,
		  jsonb_build_object('structuredIdentityRemoved', true, 'databaseRole', current_user)
		FROM changed`, studentID)
	if err != nil {
		return out, err
	}
	if err := tx.Commit(ctx); err != nil {
		return out, err
	}
	out.Applied = true
	return out, nil
}
