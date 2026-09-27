package maintenance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"quizzivy/internal/platform/db"
)

// WindowLockKey is the advisory-lock key, in the 73819 namespace, that orders
// scheduling a window against starting an attempt (docs/plan/20-data-model.md).
const WindowLockKey = 40

// ScheduleReport is what window-schedule did, or would do: the window, and how
// many running attempts and open assignments it moves by the window's length.
type ScheduleReport struct {
	WindowID    string    `json:"windowId,omitempty"`
	StartsAt    time.Time `json:"startsAt"`
	EndsAt      time.Time `json:"endsAt"`
	Attempts    int64     `json:"attempts"`
	Assignments int64     `json:"assignments"`
	Applied     bool      `json:"applied"`
}

// Window is a scheduled maintenance window as window-list prints it.
type Window struct {
	ID          string     `json:"id"`
	StartsAt    time.Time  `json:"startsAt"`
	EndsAt      time.Time  `json:"endsAt"`
	Active      bool       `json:"active"`
	CreatedBy   string     `json:"createdBy"`
	CreatedAt   time.Time  `json:"createdAt"`
	CancelledAt *time.Time `json:"cancelledAt,omitempty"`
}

// WindowReport is what window-cancel or window-end did, or would do.
type WindowReport struct {
	Window  Window `json:"window"`
	Applied bool   `json:"applied"`
}

const movingAttempts = `SELECT count(id) FROM app.attempts
 WHERE status = 'in_progress' AND deadline_at > $1`

const movingAssignments = `SELECT count(id) FROM app.assignments
 WHERE closed_at IS NULL AND published_at IS NOT NULL AND closes_at >= $1 AND closes_at < $2`

// ScheduleWindow schedules a window from startsAt to endsAt, no earlier than a
// minute ago, and in the same statement extends by its length the deadline of
// every running attempt that would still be running when it starts and the
// close of every published, open assignment that would close inside it.
// Every change is audited as System, with no actor. A dry run reports the
// counts and refusals and changes nothing, taking no lock. Applying holds the
// maintenance-windows advisory lock, which the start guard takes shared, so
// no attempt starts between the check and the write.
func ScheduleWindow(ctx context.Context, conn db.Conn, startsAt, endsAt time.Time, apply bool) (ScheduleReport, error) {
	out := ScheduleReport{StartsAt: startsAt, EndsAt: endsAt}
	if !endsAt.After(startsAt) {
		return out, errors.New("the window must end after it starts")
	}
	if endsAt.Sub(startsAt) > 12*time.Hour {
		return out, errors.New("a window may last at most 12 hours")
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if apply {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(73819, $1)`, WindowLockKey); err != nil {
			return out, fmt.Errorf("lock maintenance windows: %w", err)
		}
	}
	var tooEarly bool
	if err := tx.QueryRow(ctx, `SELECT $1::timestamptz < now() - interval '1 minute'`, startsAt).Scan(&tooEarly); err != nil {
		return out, err
	}
	if tooEarly {
		return out, errors.New("the window may start no earlier than a minute ago")
	}
	if err := refuseOverlap(ctx, tx, startsAt, endsAt); err != nil {
		return out, err
	}
	if !apply {
		if err := tx.QueryRow(ctx, movingAttempts, startsAt).Scan(&out.Attempts); err != nil {
			return out, err
		}
		err := tx.QueryRow(ctx, movingAssignments, startsAt, endsAt).Scan(&out.Assignments)
		return out, err
	}
	err = tx.QueryRow(ctx, `
		WITH scheduled AS (
		  INSERT INTO app.maintenance_windows (starts_at, ends_at)
		  VALUES ($1, $2)
		  RETURNING id, starts_at, ends_at, ends_at - starts_at AS length
		), attempts AS (
		  UPDATE app.attempts a
		     SET deadline_at = a.deadline_at + w.length
		    FROM scheduled w
		   WHERE a.status = 'in_progress' AND a.deadline_at > w.starts_at
		  RETURNING a.id, old.deadline_at AS before, new.deadline_at AS after, w.id AS window_id
		), assignments AS (
		  UPDATE app.assignments s
		     SET closes_at = s.closes_at + w.length
		    FROM scheduled w
		   WHERE s.closed_at IS NULL AND s.published_at IS NOT NULL
		     AND s.closes_at >= w.starts_at AND s.closes_at < w.ends_at
		  RETURNING s.id, old.closes_at AS before, new.closes_at AS after, w.id AS window_id
		), audited AS (
		  INSERT INTO app.audit_log (action, entity, entity_id, diff)
		  SELECT 'attempt.extended', 'attempt', id,
		         jsonb_build_object('deadline_at', jsonb_build_object('old', before, 'new', after),
		           'reason', 'maintenance', 'windowId', window_id, 'databaseRole', current_user)
		    FROM attempts
		  UNION ALL
		  SELECT 'assignment.extended', 'assignment', id,
		         jsonb_build_object('closes_at', jsonb_build_object('old', before, 'new', after),
		           'reason', 'maintenance', 'windowId', window_id, 'databaseRole', current_user)
		    FROM assignments
		  UNION ALL
		  SELECT 'maintenance.scheduled', 'maintenance_window', id,
		         jsonb_build_object('startsAt', starts_at, 'endsAt', ends_at, 'databaseRole', current_user)
		    FROM scheduled
		  RETURNING id
		)
		SELECT (SELECT id::text FROM scheduled),
		       (SELECT count(id) FROM attempts),
		       (SELECT count(id) FROM assignments)`, startsAt, endsAt).
		Scan(&out.WindowID, &out.Attempts, &out.Assignments)
	if err != nil {
		return out, readable(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return out, err
	}
	out.Applied = true
	return out, nil
}

// ListWindows returns every window that has not ended and was not cancelled,
// earliest first.
func ListWindows(ctx context.Context, conn db.Querier) ([]Window, error) {
	rows, err := conn.Query(ctx, `SELECT `+windowColumns+` FROM app.maintenance_windows
		 WHERE cancelled_at IS NULL AND ends_at > now()
		 ORDER BY starts_at`)
	if err != nil {
		return nil, err
	}
	windows, err := pgx.CollectRows(rows, scanWindow)
	if err != nil {
		return nil, err
	}
	if windows == nil {
		windows = []Window{}
	}
	return windows, nil
}

// CancelWindow cancels a window that has not ended. The extensions it granted
// stay: a deadline that moved does not move back.
func CancelWindow(ctx context.Context, conn db.Conn, windowID string, apply bool) (WindowReport, error) {
	return changeWindow(ctx, conn, windowID, apply, windowChange{
		state:  `cancelled_at IS NULL AND ends_at > now()`,
		refuse: "only a window that has not ended can be cancelled",
		set:    `cancelled_at = now()`,
		action: "maintenance.cancelled",
	})
}

// EndWindow ends an active window now. The extensions it granted stay.
func EndWindow(ctx context.Context, conn db.Conn, windowID string, apply bool) (WindowReport, error) {
	return changeWindow(ctx, conn, windowID, apply, windowChange{
		state:  `cancelled_at IS NULL AND starts_at < now() AND ends_at > now()`,
		refuse: "only an active window can be ended",
		set:    `ends_at = now()`,
		action: "maintenance.ended",
	})
}

type windowChange struct {
	state  string
	refuse string
	set    string
	action string
}

const windowColumns = `id::text, starts_at, ends_at, starts_at <= now(), created_by, created_at, cancelled_at`

func changeWindow(ctx context.Context, conn db.Conn, windowID string, apply bool, change windowChange) (WindowReport, error) {
	var out WindowReport
	if _, err := uuid.Parse(windowID); err != nil {
		return out, errors.New("-window must be a window UUID")
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(73819, $1)`, WindowLockKey); err != nil {
		return out, fmt.Errorf("lock maintenance windows: %w", err)
	}
	rows, err := tx.Query(ctx, `SELECT `+windowColumns+` FROM app.maintenance_windows
		 WHERE id = $1 AND `+change.state+` FOR UPDATE`, windowID)
	if err != nil {
		return out, err
	}
	out.Window, err = pgx.CollectExactlyOneRow(rows, scanWindow)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, errors.New(change.refuse)
	}
	if err != nil {
		return out, err
	}
	if !apply {
		return out, nil
	}
	rows, err = tx.Query(ctx, `
		WITH changed AS (
		  UPDATE app.maintenance_windows SET `+change.set+`
		   WHERE id = $1
		  RETURNING `+windowColumns+`
		), audited AS (
		  INSERT INTO app.audit_log (action, entity, entity_id, diff)
		  SELECT $2, 'maintenance_window', id::uuid,
		         jsonb_build_object('startsAt', starts_at, 'endsAt', ends_at,
		           'cancelledAt', cancelled_at, 'databaseRole', current_user)
		    FROM changed
		  RETURNING id
		)
		SELECT `+windowColumns+` FROM changed`, windowID, change.action)
	if err != nil {
		return out, err
	}
	if out.Window, err = pgx.CollectExactlyOneRow(rows, scanWindow); err != nil {
		return out, err
	}
	if err := tx.Commit(ctx); err != nil {
		return out, err
	}
	out.Applied = true
	return out, nil
}

func refuseOverlap(ctx context.Context, tx pgx.Tx, startsAt, endsAt time.Time) error {
	rows, err := tx.Query(ctx, `SELECT `+windowColumns+` FROM app.maintenance_windows
		 WHERE cancelled_at IS NULL AND tstzrange(starts_at, ends_at) && tstzrange($1, $2)
		 ORDER BY starts_at LIMIT 1`, startsAt, endsAt)
	if err != nil {
		return err
	}
	clash, err := pgx.CollectRows(rows, scanWindow)
	if err != nil {
		return err
	}
	if len(clash) > 0 {
		return fmt.Errorf("the window overlaps window %s (%s to %s); cancel or end it first",
			clash[0].ID, clash[0].StartsAt.UTC().Format(time.RFC3339), clash[0].EndsAt.UTC().Format(time.RFC3339))
	}
	return nil
}

func readable(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23P01" {
		return errors.New("the window overlaps another window scheduled at the same time; list them and try again")
	}
	return err
}

func scanWindow(row pgx.CollectableRow) (Window, error) {
	var w Window
	err := row.Scan(&w.ID, &w.StartsAt, &w.EndsAt, &w.Active, &w.CreatedBy, &w.CreatedAt, &w.CancelledAt)
	return w, err
}
