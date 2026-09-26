//go:build integration

package maintenance_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"quizzivy/internal/core/maintenance"
	"quizzivy/internal/platform/db"
)

type world struct {
	ctx context.Context
	tx  pgx.Tx
	now time.Time
}

func newWorld(t *testing.T) world {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, db.TestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	var now time.Time
	if err := tx.QueryRow(ctx, `SELECT now()`).Scan(&now); err != nil {
		t.Fatal(err)
	}
	return world{ctx: ctx, tx: tx, now: now}
}

func (w world) assignment(t *testing.T, closesAt time.Time) string {
	t.Helper()
	teacher := uuid.NewString()
	var id string
	err := w.tx.QueryRow(w.ctx, `
	 WITH teacher AS (
	   INSERT INTO app.users(email,full_name,role) VALUES($1 || '@example.com','Teacher','admin') RETURNING id
	 ), test AS (
	   INSERT INTO app.tests(title,status,current_version,created_by)
	   SELECT 'Window fixture','published',1,id FROM teacher RETURNING id,created_by
	 ), version AS (
	   INSERT INTO app.test_versions(test_id,version,total_points,published_by)
	   SELECT id,1,1,created_by FROM test RETURNING id,test_id,published_by
	 )
	 INSERT INTO app.assignments(test_id,test_version_id,opens_at,closes_at,duration_minutes,created_by,published_at)
	 SELECT test_id,id,$2::timestamptz - interval '1 hour',$3,45,published_by,$2 FROM version
	 RETURNING id::text`, teacher, w.now, closesAt).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (w world) attempt(t *testing.T, assignment string, deadline time.Time) string {
	t.Helper()
	student := uuid.NewString()
	var id string
	err := w.tx.QueryRow(w.ctx, `
	 WITH student AS (
	   INSERT INTO app.users(id,email,full_name,role) VALUES($1::uuid,$1::text || '@example.com','Student','student') RETURNING id
	 )
	 INSERT INTO app.attempts(assignment_id,test_version_id,student_id,attempt_no,status,session_id,shuffle_seed,beacon_token_hash,started_at,deadline_at)
	 SELECT a.id,a.test_version_id,student.id,1,'in_progress',uuidv7(),1,sha256('fixture'::bytea),$3::timestamptz - interval '1 minute',$4
	   FROM app.assignments a, student WHERE a.id = $2::uuid
	 RETURNING id::text`, student, assignment, w.now, deadline).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (w world) deadline(t *testing.T, attempt string) time.Time {
	t.Helper()
	var at time.Time
	if err := w.tx.QueryRow(w.ctx, `SELECT deadline_at FROM app.attempts WHERE id = $1`, attempt).Scan(&at); err != nil {
		t.Fatal(err)
	}
	return at
}

func (w world) closes(t *testing.T, assignment string) time.Time {
	t.Helper()
	var at time.Time
	if err := w.tx.QueryRow(w.ctx, `SELECT closes_at FROM app.assignments WHERE id = $1`, assignment).Scan(&at); err != nil {
		t.Fatal(err)
	}
	return at
}

func (w world) schedule(t *testing.T, startsAt, endsAt time.Time) maintenance.ScheduleReport {
	t.Helper()
	report, err := maintenance.ScheduleWindow(w.ctx, w.tx, startsAt, endsAt, true)
	if err != nil {
		t.Fatalf("ScheduleWindow: %v", err)
	}
	return report
}

func far(w world, days int) time.Time {
	return w.now.Add(time.Duration(3650+days) * 24 * time.Hour).Truncate(time.Second)
}

func TestARunningAttemptIsExtendedOnlyIfItWouldStillBeRunning(t *testing.T) {
	w := newWorld(t)
	start := far(w, 1)
	end := start.Add(90 * time.Minute)
	assignment := w.assignment(t, start.Add(24*time.Hour))
	overlapping := w.attempt(t, assignment, start.Add(10*time.Minute))
	finishedBefore := w.attempt(t, assignment, start.Add(-10*time.Minute))

	report := w.schedule(t, start, end)

	if got := w.deadline(t, overlapping); !got.Equal(start.Add(100 * time.Minute)) {
		t.Errorf("an attempt running into the window has deadline %v, want %v", got, start.Add(100*time.Minute))
	}
	if got := w.deadline(t, finishedBefore); !got.Equal(start.Add(-10 * time.Minute)) {
		t.Errorf("an attempt ending before the window moved to %v", got)
	}
	if report.WindowID == "" || !report.Applied || report.Attempts < 1 {
		t.Errorf("report = %+v", report)
	}
}

func TestAnAssignmentIsExtendedOnlyIfItClosesInsideTheWindow(t *testing.T) {
	w := newWorld(t)
	start := far(w, 2)
	end := start.Add(time.Hour)
	inside := w.assignment(t, start.Add(20*time.Minute))
	after := w.assignment(t, end.Add(time.Minute))
	atTheEnd := w.assignment(t, end)

	w.schedule(t, start, end)

	if got := w.closes(t, inside); !got.Equal(start.Add(80 * time.Minute)) {
		t.Errorf("an assignment closing inside the window closes at %v, want %v", got, start.Add(80*time.Minute))
	}
	if got := w.closes(t, after); !got.Equal(end.Add(time.Minute)) {
		t.Errorf("an assignment closing after the window moved to %v", got)
	}
	if got := w.closes(t, atTheEnd); !got.Equal(end) {
		t.Errorf("an assignment closing as the window ends moved to %v", got)
	}
}

func TestExtensionsAreAuditedAsSystem(t *testing.T) {
	w := newWorld(t)
	start := far(w, 3)
	end := start.Add(time.Hour)
	assignment := w.assignment(t, start.Add(30*time.Minute))
	attempt := w.attempt(t, assignment, start.Add(5*time.Minute))

	report := w.schedule(t, start, end)

	for _, row := range []struct{ action, entity, id, column string }{
		{"attempt.extended", "attempt", attempt, "deadline_at"},
		{"assignment.extended", "assignment", assignment, "closes_at"},
		{"maintenance.scheduled", "maintenance_window", report.WindowID, ""},
	} {
		var actor *string
		var reason, window string
		var moved bool
		err := w.tx.QueryRow(w.ctx, `
			SELECT actor_user_id::text, coalesce(diff->>'reason', ''), coalesce(diff->>'windowId', ''),
			       $3 = '' OR (diff->$3->>'old') IS DISTINCT FROM (diff->$3->>'new')
			  FROM app.audit_log WHERE action = $1 AND entity_id = $2::uuid`,
			row.action, row.id, row.column).Scan(&actor, &reason, &window, &moved)
		if err != nil {
			t.Fatalf("%s for %s: %v", row.action, row.id, err)
		}
		if actor != nil {
			t.Errorf("%s names actor %s; a maintenance change is the System's", row.action, *actor)
		}
		if row.column != "" && (reason != "maintenance" || window != report.WindowID || !moved) {
			t.Errorf("%s diff: reason %q, window %q, moved %v", row.action, reason, window, moved)
		}
	}
}

func TestAnOverlappingWindowIsRefusedReadably(t *testing.T) {
	w := newWorld(t)
	start := far(w, 4)
	first := w.schedule(t, start, start.Add(time.Hour))

	_, err := maintenance.ScheduleWindow(w.ctx, w.tx, start.Add(30*time.Minute), start.Add(2*time.Hour), true)
	if err == nil {
		t.Fatal("an overlapping window was scheduled")
	}
	if !strings.Contains(err.Error(), "overlaps window "+first.WindowID) {
		t.Errorf("error = %q, want it to name the window it overlaps", err)
	}

	if _, err := maintenance.ScheduleWindow(w.ctx, w.tx, start.Add(time.Hour), start.Add(2*time.Hour), true); err != nil {
		t.Errorf("a window starting as the other ends was refused: %v", err)
	}
}

func TestAWindowCannotStartInThePast(t *testing.T) {
	w := newWorld(t)
	_, err := maintenance.ScheduleWindow(w.ctx, w.tx, w.now.Add(-2*time.Minute), w.now.Add(time.Hour), true)
	if err == nil || !strings.Contains(err.Error(), "no earlier than a minute ago") {
		t.Errorf("a window starting two minutes ago: %v", err)
	}
}

func TestTheDryRunChangesNothing(t *testing.T) {
	w := newWorld(t)
	start := far(w, 5)
	assignment := w.assignment(t, start.Add(10*time.Minute))
	attempt := w.attempt(t, assignment, start.Add(5*time.Minute))

	report, err := maintenance.ScheduleWindow(w.ctx, w.tx, start, start.Add(time.Hour), false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Applied || report.WindowID != "" || report.Attempts < 1 || report.Assignments < 1 {
		t.Errorf("dry run report = %+v", report)
	}
	if got := w.deadline(t, attempt); !got.Equal(start.Add(5 * time.Minute)) {
		t.Errorf("the dry run moved a deadline to %v", got)
	}
	if got := w.closes(t, assignment); !got.Equal(start.Add(10 * time.Minute)) {
		t.Errorf("the dry run moved a close to %v", got)
	}
	var windows int
	if err := w.tx.QueryRow(w.ctx, `SELECT count(id) FROM app.maintenance_windows WHERE starts_at = $1`, start).Scan(&windows); err != nil {
		t.Fatal(err)
	}
	if windows != 0 {
		t.Errorf("the dry run wrote %d windows", windows)
	}
}

func TestCancellingKeepsTheExtensions(t *testing.T) {
	w := newWorld(t)
	start := far(w, 6)
	assignment := w.assignment(t, start.Add(24*time.Hour))
	attempt := w.attempt(t, assignment, start.Add(5*time.Minute))
	report := w.schedule(t, start, start.Add(time.Hour))

	cancelled, err := maintenance.CancelWindow(w.ctx, w.tx, report.WindowID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !cancelled.Applied || cancelled.Window.CancelledAt == nil {
		t.Errorf("cancel report = %+v", cancelled)
	}
	if got := w.deadline(t, attempt); !got.Equal(start.Add(65 * time.Minute)) {
		t.Errorf("cancelling moved the deadline back to %v", got)
	}
	listed, err := maintenance.ListWindows(w.ctx, w.tx)
	if err != nil {
		t.Fatal(err)
	}
	for _, window := range listed {
		if window.ID == report.WindowID {
			t.Error("a cancelled window is still listed")
		}
	}
	if _, err := maintenance.CancelWindow(w.ctx, w.tx, report.WindowID, true); err == nil {
		t.Error("a cancelled window was cancelled again")
	}
}

func TestEndingKeepsTheExtensionsAndOnlyEndsAnActiveWindow(t *testing.T) {
	w := newWorld(t)
	start := w.now.Add(-30 * time.Second)
	assignment := w.assignment(t, w.now.Add(24*time.Hour))
	attempt := w.attempt(t, assignment, w.now.Add(10*time.Minute))
	report := w.schedule(t, start, start.Add(time.Hour))

	listed, err := maintenance.ListWindows(w.ctx, w.tx)
	if err != nil {
		t.Fatal(err)
	}
	var active bool
	for _, window := range listed {
		active = active || (window.ID == report.WindowID && window.Active)
	}
	if !active {
		t.Errorf("the window running now is not listed as active: %+v", listed)
	}

	ended, err := maintenance.EndWindow(w.ctx, w.tx, report.WindowID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !ended.Applied || !ended.Window.EndsAt.Equal(w.now) {
		t.Errorf("end report = %+v, want it to end at %v", ended, w.now)
	}
	if got := w.deadline(t, attempt); !got.Equal(w.now.Add(70 * time.Minute)) {
		t.Errorf("ending moved the deadline to %v", got)
	}

	future := w.schedule(t, far(w, 7), far(w, 7).Add(time.Hour))
	if _, err := maintenance.EndWindow(w.ctx, w.tx, future.WindowID, true); err == nil {
		t.Error("a window that has not started was ended")
	}
}
