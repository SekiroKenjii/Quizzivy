// Package maintenance implements the operator's privileged commands, run outside the API process under the quizzivy_maintenance role: the retention sweep, integrity retention, account anonymization, maintenance windows, join-code re-keying and the AI credits console.
package maintenance

import (
	"context"
	"errors"
	"time"

	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/retention"
)

// RetentionReport describes one bounded batch using the database's UTC calendar cutoff.
type RetentionReport struct {
	Cutoff  time.Time `json:"cutoff"`
	Rows    int64     `json:"rows"`
	Applied bool      `json:"applied"`
}

const maxBatch = 10000

func checkBatch(batch int) error {
	if batch < 1 || batch > maxBatch {
		return errors.New("batch must be between 1 and 10000")
	}
	return nil
}

func cutoffBefore(ctx context.Context, conn db.Querier, months int) (time.Time, error) {
	var cutoff time.Time
	err := conn.QueryRow(ctx,
		`SELECT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC' - make_interval(months => $1)) AT TIME ZONE 'UTC'`, months).Scan(&cutoff)
	return cutoff, err
}

var eventsDue = `FROM app.attempt_events e
 JOIN app.attempts at ON at.id = e.attempt_id ` + retention.AttemptJoins + `
 WHERE e.received_at < $1 AND ` + retention.ClosedBefore("$1")

var retentionCandidates = `SELECT e.id ` + eventsDue + `
 ORDER BY e.received_at,e.id LIMIT $2`

// RetainEvents counts or deletes one bounded batch of integrity events once both their receipt and the close of their assignment for the student, an override included, are months old, on the UTC calendar.
// Apply writes as the caller's role; no application-role privileges are changed.
func RetainEvents(ctx context.Context, conn db.Querier, months int, apply bool, batch int) (RetentionReport, error) {
	var out RetentionReport
	if err := checkBatch(batch); err != nil {
		return out, err
	}
	cutoff, err := cutoffBefore(ctx, conn, months)
	if err != nil {
		return out, err
	}
	out.Cutoff = cutoff
	if !apply {
		err = conn.QueryRow(ctx, `SELECT count(id) FROM (`+retentionCandidates+`) AS candidates`, cutoff, batch).Scan(&out.Rows)
		return out, err
	}
	out.Rows, err = deleteEvents(ctx, conn, cutoff, batch)
	out.Applied = err == nil
	return out, err
}

func deleteEvents(ctx context.Context, conn db.Querier, cutoff time.Time, batch int) (int64, error) {
	var rows int64
	err := conn.QueryRow(ctx, `
		WITH candidates AS (`+retentionCandidates+`
		), removed AS (
		  DELETE FROM app.attempt_events e USING candidates c WHERE e.id = c.id RETURNING e.id
		), logged AS (
		  INSERT INTO app.audit_log (action, entity, diff)
		  SELECT 'integrity.retained', 'attempt_events',
		    jsonb_build_object('cutoff', $1::timestamptz, 'deletedRows', count(id), 'databaseRole', current_user)
		  FROM removed HAVING count(id) > 0
		)
		SELECT count(id) FROM removed`, cutoff, batch).Scan(&rows)
	return rows, err
}
