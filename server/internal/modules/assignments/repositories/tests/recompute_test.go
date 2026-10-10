//go:build integration

package repositories_test

import (
	"context"
	"testing"
	"time"

	"quizzivy/internal/modules/assignments/domain"
)

func (w *reachWorld) sitting(t *testing.T, assignment, student string, startedAgo time.Duration, deadline time.Time, status string) string {
	t.Helper()
	submitted := "NULL"
	voidReason := "NULL"
	switch status {
	case "submitted":
		submitted = "now()"
	case "voided":
		voidReason = "'mất điện'"
	}
	return w.id(t, `
		INSERT INTO app.attempts
		  (assignment_id, test_version_id, student_id, attempt_no, status, session_id,
		   shuffle_seed, beacon_token_hash, started_at, deadline_at, submitted_at, void_reason)
		VALUES ($1::uuid, (SELECT test_version_id FROM app.assignments WHERE id = $1::uuid), $2::uuid,
		        (SELECT coalesce(max(attempt_no), 0) + 1 FROM app.attempts WHERE assignment_id = $1::uuid AND student_id = $2::uuid),
		        $3::app.attempt_status, gen_random_uuid(), 1, sha256('b'::bytea),
		        now() - make_interval(secs => $4::float8), $5, `+submitted+`, `+voidReason+`)
		RETURNING id::text`,
		assignment, student, status, startedAgo.Seconds(), deadline)
}

func (w *reachWorld) deadlineOf(t *testing.T, attempt string) time.Time {
	t.Helper()
	var deadline time.Time
	if err := w.tx.QueryRow(context.Background(), `SELECT deadline_at FROM app.attempts WHERE id = $1::uuid`, attempt).Scan(&deadline); err != nil {
		t.Fatal(err)
	}
	return deadline
}

func (w *reachWorld) startedOf(t *testing.T, attempt string) time.Time {
	t.Helper()
	var started time.Time
	if err := w.tx.QueryRow(context.Background(), `SELECT started_at FROM app.attempts WHERE id = $1::uuid`, attempt).Scan(&started); err != nil {
		t.Fatal(err)
	}
	return started
}

func (w *reachWorld) movedAttempts(t *testing.T, assignment string) int {
	t.Helper()
	var n int
	if err := w.tx.QueryRow(context.Background(), `
		SELECT count(*) FROM app.audit_log
		 WHERE action = 'attempt.extended' AND entity = 'attempt' AND diff->>'assignmentId' = $1`, assignment).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (w *reachWorld) closingSoon(t *testing.T, owner, class string, closesIn time.Duration, duration int) string {
	t.Helper()
	in := input(w.versionA, []string{class}, nil)
	now := time.Now()
	in.OpensAt, in.ClosesAt, in.DurationMin = now.Add(-time.Hour), now.Add(closesIn), duration
	return w.create(t, as(owner, false), in)
}

func TestExtendingMovesTheDeadlinesTheCloseHeldDown(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	assignment := w.closingSoon(t, w.a, w.classA, 20*time.Minute, 45)
	other := w.closingSoon(t, w.a, w.classA, 20*time.Minute, 45)
	cut, roomy, delivered, voided, beyond, elsewhere := w.classmate(t, w.classA, w.a), w.classmate(t, w.classA, w.a),
		w.classmate(t, w.classA, w.a), w.classmate(t, w.classA, w.a), w.classmate(t, w.classA, w.a), w.classmate(t, w.classA, w.a)
	closes := w.closesAt(t, assignment)
	nowish := time.Now()

	cutAttempt := w.sitting(t, assignment, cut, 5*time.Minute, closes, "in_progress")
	roomyAttempt := w.sitting(t, assignment, roomy, 30*time.Minute, nowish.Add(15*time.Minute), "in_progress")
	deliveredAttempt := w.sitting(t, assignment, delivered, 5*time.Minute, closes, "submitted")
	voidedAttempt := w.sitting(t, assignment, voided, 5*time.Minute, closes, "voided")
	pushedPast := closes.Add(6 * time.Hour)
	beyondAttempt := w.sitting(t, assignment, beyond, 5*time.Minute, pushedPast, "in_progress")
	elsewhereAttempt := w.sitting(t, other, elsewhere, 5*time.Minute, w.closesAt(t, other), "in_progress")
	roomyBefore, elsewhereBefore := w.deadlineOf(t, roomyAttempt), w.deadlineOf(t, elsewhereAttempt)

	req := as(w.a, false)
	req.ID = assignment
	req.IP, req.UserAgent = "203.0.113.7", "go-test"
	if _, err := w.store.Extend(ctx, req, 120, false, time.Now()); err != nil {
		t.Fatalf("extend: %v", err)
	}

	if got, want := w.deadlineOf(t, cutAttempt), w.startedOf(t, cutAttempt).Add(45*time.Minute); !got.Equal(want) {
		t.Errorf("the attempt the close held down has deadline %v, want its start plus 45 minutes, %v", got, want)
	}
	for name, c := range map[string]struct {
		attempt string
		want    time.Time
	}{
		"an attempt whose time limit, not the close, set its deadline": {roomyAttempt, roomyBefore},
		"an attempt already handed in":                                 {deliveredAttempt, closes},
		"a voided attempt":                                             {voidedAttempt, closes},
		"a deadline a maintenance window had pushed past the rule":    {beyondAttempt, pushedPast},
		"an attempt on another assignment":                             {elsewhereAttempt, elsewhereBefore},
	} {
		if got := w.deadlineOf(t, c.attempt); !got.Equal(c.want) {
			t.Errorf("%s moved to %v, want it kept at %v", name, got, c.want)
		}
	}

	var moved int
	var actor, reason, assignmentID string
	if err := w.tx.QueryRow(ctx, `
		SELECT count(*), min(actor_user_id::text), min(diff->>'reason'), min(diff->>'assignmentId')
		  FROM app.audit_log
		 WHERE action = 'attempt.extended' AND entity = 'attempt' AND entity_id = $1::uuid`, cutAttempt).Scan(&moved, &actor, &reason, &assignmentID); err != nil {
		t.Fatal(err)
	}
	if moved != 1 || actor != w.a || reason != "assignment_extended" || assignmentID != assignment {
		t.Errorf("the moved attempt's audit = %d rows by %s for %q on %s; want one by the teacher, for assignment_extended, on %s", moved, actor, reason, assignmentID, assignment)
	}
	var unmoved int
	if err := w.tx.QueryRow(ctx, `
		SELECT count(*) FROM app.audit_log WHERE action = 'attempt.extended' AND entity_id = ANY($1::uuid[])`,
		[]string{roomyAttempt, deliveredAttempt, voidedAttempt, beyondAttempt, elsewhereAttempt}).Scan(&unmoved); err != nil {
		t.Fatal(err)
	}
	if unmoved != 0 {
		t.Errorf("%d audit entries for attempts that did not move", unmoved)
	}
}

func TestAnOverridesCloseIsNeverTakenBackByAnExtension(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	assignment := w.closingSoon(t, w.a, w.classA, 20*time.Minute, 45)
	student := w.classmate(t, w.classA, w.a)
	req := as(w.a, false)
	req.ID = assignment
	until := after(3 * time.Hour)
	if _, err := w.store.SetOverrides(ctx, req, domain.OverrideInput{
		StudentIDs: []string{student}, ClosesAt: until, DurationMin: ptr(120), Reason: "thêm giờ", Now: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	attempt := w.sitting(t, assignment, student, 5*time.Minute, time.Now().Add(115*time.Minute), "in_progress")
	before := w.deadlineOf(t, attempt)

	if _, err := w.store.Extend(ctx, req, 30, false, time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := w.deadlineOf(t, attempt); !got.Equal(before) {
		t.Errorf("an extension that closes before the student's own close moved their deadline to %v from %v", got, before)
	}
}

func TestAnOverrideMovesTheDeadlinesOfTheStudentsNamedAndNoOthers(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	assignment := w.closingSoon(t, w.a, w.classA, 20*time.Minute, 45)
	named, bystander, quicker := w.classmate(t, w.classA, w.a), w.classmate(t, w.classA, w.a), w.classmate(t, w.classA, w.a)
	closes := w.closesAt(t, assignment)
	namedAttempt := w.sitting(t, assignment, named, 5*time.Minute, closes, "in_progress")
	bystanderAttempt := w.sitting(t, assignment, bystander, 5*time.Minute, closes, "in_progress")
	quickerAttempt := w.sitting(t, assignment, quicker, 5*time.Minute, closes, "in_progress")

	req := as(w.a, false)
	req.ID = assignment
	req.IP = "203.0.113.9"
	if _, err := w.store.SetOverrides(ctx, req, domain.OverrideInput{
		StudentIDs: []string{named}, ClosesAt: after(2 * time.Hour), DurationMin: ptr(90), Reason: "đau ốm", Now: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	if got, want := w.deadlineOf(t, namedAttempt), w.startedOf(t, namedAttempt).Add(90*time.Minute); !got.Equal(want) {
		t.Errorf("the named student's deadline is %v, want their start plus 90 minutes, %v", got, want)
	}
	if got := w.deadlineOf(t, bystanderAttempt); !got.Equal(closes) {
		t.Errorf("a student the request did not name has deadline %v, want it kept at %v", got, closes)
	}

	if _, err := w.store.SetOverrides(ctx, req, domain.OverrideInput{
		StudentIDs: []string{quicker}, DurationMin: ptr(5), Reason: "ngắn hơn", Now: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	if got := w.deadlineOf(t, quickerAttempt); !got.Equal(closes) {
		t.Errorf("a shorter time limit moved a deadline to %v, want it kept at %v", got, closes)
	}

	var reason, actor string
	if err := w.tx.QueryRow(ctx, `
		SELECT diff->>'reason', actor_user_id::text FROM app.audit_log
		 WHERE action = 'attempt.extended' AND entity_id = $1::uuid`, namedAttempt).Scan(&reason, &actor); err != nil {
		t.Fatal(err)
	}
	if reason != "student_override" || actor != w.a {
		t.Errorf("the moved attempt's audit says %q by %s, want student_override by the teacher", reason, actor)
	}
}

func TestAnOverrideThatNeverLetsAnAttemptRunLongerLeavesItAlone(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	assignment := w.closingSoon(t, w.a, w.classA, 2*time.Hour, 45)
	student := w.classmate(t, w.classA, w.a)
	attempt := w.sitting(t, assignment, student, 5*time.Minute, time.Now().Add(40*time.Minute), "in_progress")
	before := w.deadlineOf(t, attempt)
	req := as(w.a, false)
	req.ID = assignment

	if _, err := w.store.SetOverrides(ctx, req, domain.OverrideInput{
		StudentIDs: []string{student}, ExtraAttempts: ptr(2), Reason: "thêm lượt", Now: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	if got := w.deadlineOf(t, attempt); !got.Equal(before) {
		t.Errorf("extra attempts moved the deadline of an attempt to %v from %v", got, before)
	}
	if n := w.movedAttempts(t, assignment); n != 0 {
		t.Errorf("an override that moved nothing wrote %d attempt audit entries", n)
	}
}

func TestRemovingAnOverrideNeverShortensAnAttemptInProgress(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	assignment := w.closingSoon(t, w.a, w.classA, 20*time.Minute, 45)
	student := w.classmate(t, w.classA, w.a)
	req := as(w.a, false)
	req.ID = assignment
	if _, err := w.store.SetOverrides(ctx, req, domain.OverrideInput{
		StudentIDs: []string{student}, DurationMin: ptr(120), ClosesAt: after(3 * time.Hour), Reason: "thêm giờ", Now: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	attempt := w.sitting(t, assignment, student, 5*time.Minute, time.Now().Add(115*time.Minute), "in_progress")
	before := w.deadlineOf(t, attempt)

	if err := w.store.DeleteOverride(ctx, req, student, time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := w.deadlineOf(t, attempt); !got.Equal(before) {
		t.Errorf("removing the override moved the deadline to %v from %v", got, before)
	}
}

func TestACloseThatIsNotAheadOfTheDatabaseClockIsRefused(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	req := as(w.a, false)
	req.ID = w.mineA
	var databaseNow time.Time
	if err := w.tx.QueryRow(ctx, `SELECT now()`).Scan(&databaseNow); err != nil {
		t.Fatal(err)
	}
	for name, at := range map[string]time.Time{"in the past": databaseNow.Add(-time.Minute), "exactly now": databaseNow} {
		_, err := w.store.SetOverrides(ctx, req, domain.OverrideInput{
			StudentIDs: []string{w.studentA}, ClosesAt: &at, Reason: "mở lại", Now: databaseNow.Add(-time.Hour),
		})
		if got := fieldsOf(t, err); got["closesAt"] == "" {
			t.Errorf("a close %s: fields %v, want one on closesAt", name, got)
		}
	}
	ahead := databaseNow.Add(time.Second)
	if _, err := w.store.SetOverrides(ctx, req, domain.OverrideInput{
		StudentIDs: []string{w.studentA}, ClosesAt: &ahead, Reason: "mở lại", Now: databaseNow.Add(-time.Hour),
	}); err != nil {
		t.Errorf("a close a second ahead of the database clock: %v", err)
	}
}
