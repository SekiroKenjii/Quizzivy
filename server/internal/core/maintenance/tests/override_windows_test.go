//go:build integration

package maintenance_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"quizzivy/internal/core/maintenance"
	availabilityrepo "quizzivy/internal/modules/availability/repositories"
	"quizzivy/internal/shared/schedule"
)

func (w world) student(t *testing.T) string {
	t.Helper()
	var id string
	if err := w.tx.QueryRow(w.ctx, `
		INSERT INTO app.users(email,full_name,role_id)
		VALUES ($1 || '@example.com','Student',(SELECT id FROM app.roles WHERE builtin_key = 'student'))
		RETURNING id::text`, uuid.NewString()).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (w world) override(t *testing.T, assignment string, closesAt time.Time) string {
	t.Helper()
	student := w.student(t)
	if _, err := w.tx.Exec(w.ctx, `
		INSERT INTO app.assignment_student_overrides (assignment_id, student_id, closes_at, reason)
		VALUES ($1::uuid, $2::uuid, $3, 'Kiểm thử cửa sổ bảo trì')`, assignment, student, closesAt); err != nil {
		t.Fatal(err)
	}
	return student
}

func (w world) overrideClose(t *testing.T, assignment, student string) time.Time {
	t.Helper()
	var at time.Time
	if err := w.tx.QueryRow(w.ctx, `
		SELECT closes_at FROM app.assignment_student_overrides WHERE assignment_id = $1::uuid AND student_id = $2::uuid`,
		assignment, student).Scan(&at); err != nil {
		t.Fatal(err)
	}
	return at
}

func TestAnOverrideClosingInsideTheWindowIsExtendedWithIt(t *testing.T) {
	w := newWorld(t)
	start := far(w, 6)
	end := start.Add(time.Hour)
	assignment := w.assignment(t, start.Add(24*time.Hour))
	inside := w.override(t, assignment, start.Add(20*time.Minute))
	after := w.override(t, assignment, end.Add(time.Minute))
	atTheEnd := w.override(t, assignment, end)
	draft := w.draft(t, start.Add(24*time.Hour))
	onADraft := w.override(t, draft, start.Add(20*time.Minute))
	closedEarly := w.assignment(t, start.Add(24*time.Hour))
	if _, err := w.tx.Exec(w.ctx, `UPDATE app.assignments SET closed_at = $2 WHERE id = $1::uuid`, closedEarly, w.now); err != nil {
		t.Fatal(err)
	}
	keptIn := w.override(t, closedEarly, start.Add(40*time.Minute))

	report := w.schedule(t, start, end)

	for name, c := range map[string]struct {
		assignment, student string
		want                time.Time
	}{
		"an override closing inside the window":                   {assignment, inside, start.Add(80 * time.Minute)},
		"an override closing after the window":                    {assignment, after, end.Add(time.Minute)},
		"an override closing as the window ends":                  {assignment, atTheEnd, end},
		"an override on a draft no one can take":                  {draft, onADraft, start.Add(20 * time.Minute)},
		"an override that keeps a student in an early-closed one": {closedEarly, keptIn, start.Add(100 * time.Minute)},
	} {
		if got := w.overrideClose(t, c.assignment, c.student); !got.Equal(c.want) {
			t.Errorf("%s closes at %v, want %v", name, got, c.want)
		}
	}
	if report.Overrides < 2 {
		t.Errorf("report = %+v, want it to count the overrides it moved", report)
	}

	var actor *string
	var reason, window, student string
	var moved bool
	if err := w.tx.QueryRow(w.ctx, `
		SELECT actor_user_id::text, diff->>'reason', diff->>'windowId', diff->>'studentId',
		       (diff->'closes_at'->>'old') IS DISTINCT FROM (diff->'closes_at'->>'new')
		  FROM app.audit_log WHERE action = 'assignment.override_extended' AND entity_id = $1::uuid AND diff->>'studentId' = $2`,
		assignment, inside).Scan(&actor, &reason, &window, &student, &moved); err != nil {
		t.Fatalf("the override's audit entry: %v", err)
	}
	if actor != nil || reason != "maintenance" || window != report.WindowID || student != inside || !moved {
		t.Errorf("audit: actor %v, reason %q, window %q, student %q, moved %v", actor, reason, window, student, moved)
	}
	var untouched int
	if err := w.tx.QueryRow(w.ctx, `
		SELECT count(*) FROM app.audit_log WHERE action = 'assignment.override_extended' AND diff->>'studentId' = ANY($1::text[])`,
		[]string{after, atTheEnd, onADraft}).Scan(&untouched); err != nil {
		t.Fatal(err)
	}
	if untouched != 0 {
		t.Errorf("%d audit entries for overrides that did not move", untouched)
	}
}

func TestTheDryRunCountsOverridesAndMovesNone(t *testing.T) {
	w := newWorld(t)
	start := far(w, 7)
	assignment := w.assignment(t, start.Add(24*time.Hour))
	student := w.override(t, assignment, start.Add(10*time.Minute))

	report, err := maintenance.ScheduleWindow(w.ctx, w.tx, start, start.Add(time.Hour), false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Applied || report.Overrides < 1 {
		t.Errorf("dry run report = %+v", report)
	}
	if got := w.overrideClose(t, assignment, student); !got.Equal(start.Add(10 * time.Minute)) {
		t.Errorf("the dry run moved an override's close to %v", got)
	}
}

func TestEveryWriterOfAWindowTakesTheSameLock(t *testing.T) {
	if maintenance.WindowLockKey != schedule.WindowLockKey || availabilityrepo.WindowLockKey != schedule.WindowLockKey {
		t.Errorf("the window lock keys differ: maintenance %d, availability %d, schedule %d",
			maintenance.WindowLockKey, availabilityrepo.WindowLockKey, schedule.WindowLockKey)
	}
}
