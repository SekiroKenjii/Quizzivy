//go:build integration

package maintenance_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"math/rand/v2"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"quizzivy/internal/core/adapters"
	"quizzivy/internal/core/maintenance"
	attemptsdomain "quizzivy/internal/modules/attempts/domain"
	attemptsrepo "quizzivy/internal/modules/attempts/repositories"
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

func (w world) draft(t *testing.T, closesAt time.Time) string {
	t.Helper()
	id := w.assignment(t, closesAt)
	if _, err := w.tx.Exec(w.ctx, `UPDATE app.assignments SET published_at = NULL WHERE id = $1`, id); err != nil {
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
	draft := w.draft(t, start.Add(20*time.Minute))

	w.schedule(t, start, end)

	if got := w.closes(t, draft); !got.Equal(start.Add(20 * time.Minute)) {
		t.Errorf("a draft closing inside the window moved to %v; no one can take a draft", got)
	}

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
	start := far(w, 8)
	assignment := w.assignment(t, start.Add(24*time.Hour))
	attempt := w.attempt(t, assignment, start.Add(10*time.Minute))
	report := w.schedule(t, start, start.Add(time.Hour))

	if _, err := maintenance.EndWindow(w.ctx, w.tx, report.WindowID, true); err == nil {
		t.Error("a window that has not started was ended")
	}

	if _, err := w.tx.Exec(w.ctx,
		`UPDATE app.maintenance_windows SET starts_at = $2, ends_at = $3 WHERE id = $1`,
		report.WindowID, w.now.Add(-30*time.Second), w.now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
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
	if got := w.deadline(t, attempt); !got.Equal(start.Add(70 * time.Minute)) {
		t.Errorf("ending moved the extended deadline to %v", got)
	}
	var endAudits int
	if err := w.tx.QueryRow(w.ctx,
		`SELECT count(id) FROM app.audit_log WHERE action = 'maintenance.ended' AND entity_id = $1::uuid AND actor_user_id IS NULL`,
		report.WindowID).Scan(&endAudits); err != nil {
		t.Fatal(err)
	}
	if endAudits != 1 {
		t.Errorf("%d maintenance.ended rows, want 1", endAudits)
	}
}

type committed struct {
	pool       *pgxpool.Pool
	teacher    string
	test       string
	version    string
	assignment string
	students   []string
}

func commitAssignment(t *testing.T, base time.Time) *committed {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, db.TestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	c := &committed{pool: pool}
	t.Cleanup(func() {
		for _, q := range []string{
			`DELETE FROM app.maintenance_windows WHERE starts_at >= $1 AND starts_at < $1 + interval '30 days'`,
			`DELETE FROM app.attempts WHERE assignment_id::text = $2`,
			`DELETE FROM app.assignments WHERE id::text = $2`,
			`DELETE FROM app.test_versions WHERE test_id::text = $3`,
			`DELETE FROM app.tests WHERE id::text = $3`,
			`DELETE FROM app.users WHERE id::text = ANY($4::text[])`,
		} {
			_, _ = pool.Exec(ctx, q, base, c.assignment, c.test, append([]string{c.teacher}, c.students...))
		}
		pool.Close()
	})
	err = pool.QueryRow(ctx, `
	 WITH teacher AS (
	   INSERT INTO app.users(email,full_name,role) VALUES($1 || '@example.com','Teacher','admin') RETURNING id
	 ), test AS (
	   INSERT INTO app.tests(title,status,current_version,created_by)
	   SELECT 'Window race fixture','published',1,id FROM teacher RETURNING id,created_by
	 ), version AS (
	   INSERT INTO app.test_versions(test_id,version,total_points,published_by)
	   SELECT id,1,1,created_by FROM test RETURNING id,test_id,published_by
	 ), assignment AS (
	   INSERT INTO app.assignments(test_id,test_version_id,opens_at,closes_at,duration_minutes,created_by,published_at)
	   SELECT test_id,id,$2::timestamptz - interval '1 day',$2::timestamptz + interval '400 days',45,published_by,now() FROM version
	   RETURNING id,test_id,test_version_id,created_by
	 )
	 SELECT created_by::text,test_id::text,test_version_id::text,id::text FROM assignment`,
		uuid.NewString(), base).Scan(&c.teacher, &c.test, &c.version, &c.assignment)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func (c *committed) student(t *testing.T) string {
	t.Helper()
	var id string
	if err := c.pool.QueryRow(context.Background(), `
		INSERT INTO app.users(email,full_name,role) VALUES($1 || '@example.com','Student','student')
		RETURNING id::text`, uuid.NewString()).Scan(&id); err != nil {
		t.Fatal(err)
	}
	c.students = append(c.students, id)
	return id
}

func TestWindowScheduleSerialisesAgainstAttemptStart(t *testing.T) {
	base := time.Now().Add(40*365*24*time.Hour + time.Duration(rand.IntN(80000))*time.Hour).Truncate(time.Second)
	c := commitAssignment(t, base)
	starts := attemptsrepo.NewPostgres(db.NewContext(c.pool), adapters.AttemptStartGuard{})
	ctx := context.Background()

	for attempt := range 8 {
		windowStart := base.Add(time.Duration(attempt) * 24 * time.Hour)
		hash := sha256.Sum256([]byte(uuid.NewString()))
		in := attemptsdomain.CreateInput{
			AssignmentID:  c.assignment,
			TestVersionID: c.version,
			StudentID:     c.student(t),
			AttemptNo:     1,
			SessionID:     uuid.NewString(),
			Seed:          1,
			BeaconHash:    hash[:],
			StartedAt:     windowStart.Add(-10 * time.Minute),
			DeadlineAt:    windowStart.Add(30 * time.Minute),
		}

		var wg sync.WaitGroup
		var created attemptsdomain.AttemptRecord
		var createErr, scheduleErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			created, createErr = starts.Create(ctx, in)
		}()
		go func() {
			defer wg.Done()
			_, scheduleErr = maintenance.ScheduleWindow(ctx, c.pool, windowStart, windowStart.Add(time.Hour), true)
		}()
		wg.Wait()

		if scheduleErr != nil {
			t.Fatalf("attempt %d: schedule: %v", attempt, scheduleErr)
		}
		var refused *attemptsdomain.MaintenanceScheduledError
		switch {
		case errors.As(createErr, &refused):
		case createErr != nil:
			t.Fatalf("attempt %d: start failed with %v, want the attempt or MaintenanceScheduledError", attempt, createErr)
		default:
			var deadline time.Time
			if err := c.pool.QueryRow(ctx, `SELECT deadline_at FROM app.attempts WHERE id = $1::uuid`, created.ID).Scan(&deadline); err != nil {
				t.Fatal(err)
			}
			if want := in.DeadlineAt.Add(time.Hour); !deadline.Equal(want) {
				t.Fatalf("attempt %d: an attempt started across a new window kept deadline %v, want %v; "+
					"it will be cut off by the maintenance", attempt, deadline, want)
			}
		}
	}
}
