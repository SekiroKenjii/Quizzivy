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
	err := conn.QueryRow(ctx, `SELECT `+retention.Cutoff("$1"), months).Scan(&cutoff)
	return cutoff, err
}

var retentionCandidates = `SELECT e.id, e.received_at ` + retention.EventsDue + `
 AND (e.received_at, e.id) > ($3::timestamptz, $4::bigint)
 ORDER BY e.received_at,e.id LIMIT $2`

type eventsCursor struct {
	receivedAt time.Time
	id         int64
}

var fromTheStart = eventsCursor{}

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
		err = conn.QueryRow(ctx, `SELECT count(id) FROM (`+retentionCandidates+`) AS candidates`,
			cutoff, batch, fromTheStart.receivedAt, fromTheStart.id).Scan(&out.Rows)
		return out, err
	}
	out.Rows, _, err = deleteEvents(ctx, conn, cutoff, batch, fromTheStart)
	out.Applied = err == nil
	return out, err
}

func deleteEvents(ctx context.Context, conn db.Querier, cutoff time.Time, batch int, after eventsCursor) (int64, eventsCursor, error) {
	var rows int64
	var lastReceivedAt *time.Time
	var lastID *int64
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
		SELECT count(id),
		       (SELECT received_at FROM candidates ORDER BY received_at DESC, id DESC LIMIT 1),
		       (SELECT id FROM candidates ORDER BY received_at DESC, id DESC LIMIT 1)
		  FROM removed`, cutoff, batch, after.receivedAt, after.id).Scan(&rows, &lastReceivedAt, &lastID)
	if err != nil || lastReceivedAt == nil || lastID == nil {
		return rows, after, err
	}
	return rows, eventsCursor{receivedAt: *lastReceivedAt, id: *lastID}, nil
}
