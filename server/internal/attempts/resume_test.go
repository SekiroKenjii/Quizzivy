package attempts_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/attempts"
)

func eventKinds(t *testing.T, pool *pgxpool.Pool, attemptID string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT kind FROM app.attempt_events WHERE attempt_id = $1::uuid ORDER BY id`, attemptID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var kind string
		if err := rows.Scan(&kind); err != nil {
			t.Fatal(err)
		}
		out = append(out, kind)
	}
	return out
}

func count(t *testing.T, pool *pgxpool.Pool, q string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestResumingReturnsTheSameAttemptOnANewSession(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	svc := newService(t, pool)
	ctx := context.Background()

	first, err := svc.StartOrResume(ctx, w.assignment, w.student)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	second, err := svc.StartOrResume(ctx, w.assignment, w.student)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}

	if second.Attempt.ID != first.Attempt.ID {
		t.Fatalf("resume produced a different attempt: %s then %s", first.Attempt.ID, second.Attempt.ID)
	}
	if second.SessionID == first.SessionID {
		t.Error("the session id did not change; the superseded tab would keep writing")
	}
	if second.BeaconToken == first.BeaconToken {
		t.Error("the beacon token did not change; the superseded tab keeps append access")
	}
	// The clock does not restart on a reload.
	if !second.Attempt.DeadlineAt.Equal(first.Attempt.DeadlineAt) {
		t.Errorf("deadline moved from %v to %v", first.Attempt.DeadlineAt, second.Attempt.DeadlineAt)
	}
	if !second.Attempt.StartedAt.Equal(first.Attempt.StartedAt) {
		t.Error("startedAt moved")
	}
}

// The superseded session learns it lost on its next write, so the swap has to
// have actually landed in the row -- not only in the response.
func TestTheSupersededSessionIsNoLongerTheAttemptsSession(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	svc := newService(t, pool)
	ctx := context.Background()

	first, _ := svc.StartOrResume(ctx, w.assignment, w.student)
	second, err := svc.StartOrResume(ctx, w.assignment, w.student)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}

	stale := count(t, pool, `SELECT count(*) FROM app.attempts
	                          WHERE id = $1::uuid AND session_id = $2::uuid`,
		first.Attempt.ID, first.SessionID)
	if stale != 0 {
		t.Error("the old session is still the attempt's session")
	}
	current := count(t, pool, `SELECT count(*) FROM app.attempts
	                            WHERE id = $1::uuid AND session_id = $2::uuid`,
		first.Attempt.ID, second.SessionID)
	if current != 1 {
		t.Error("the new session is not the attempt's session")
	}
}

func TestAReloadIsAResumeAndNotATakeover(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	svc := newService(t, pool)
	ctx := context.Background()

	first, _ := svc.StartOrResume(ctx, w.assignment, w.student)
	if _, err := svc.StartOrResume(ctx, w.assignment, w.student); err != nil {
		t.Fatalf("resume: %v", err)
	}

	// A tab that sent nothing is a tab that is gone -- a crash, a closed laptop, a reload.
	if got := eventKinds(t, pool, first.Attempt.ID); len(got) != 1 || got[0] != attempts.KindResume {
		t.Fatalf("events %v, want exactly [resume]", got)
	}
}

func TestASecondDeviceOnALiveSessionIsRecordedAsATakeover(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	svc := newService(t, pool)
	ctx := context.Background()

	first, _ := svc.StartOrResume(ctx, w.assignment, w.student)

	if _, err := pool.Exec(ctx, `
		INSERT INTO app.attempt_events (attempt_id, session_id, kind, occurred_at, client_seq)
		VALUES ($1::uuid,$2::uuid,'tab_hidden',now(),0)`,
		first.Attempt.ID, first.SessionID); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.StartOrResume(ctx, w.assignment, w.student); err != nil {
		t.Fatalf("resume: %v", err)
	}

	kinds := map[string]int{}
	for _, k := range eventKinds(t, pool, first.Attempt.ID) {
		kinds[k]++
	}
	if kinds[attempts.KindSessionTakeover] != 1 {
		t.Errorf("%d session_takeover events, want 1 (kinds: %v)", kinds[attempts.KindSessionTakeover], kinds)
	}
	if kinds[attempts.KindResume] != 1 {
		t.Errorf("%d resume events, want 1 (kinds: %v)", kinds[attempts.KindResume], kinds)
	}
}

// The bug the liveness index's predicate exists to prevent. Two reloads in a
// row must not have the second read the first's own `resume` as a live rival.
func TestBackToBackReloadsDoNotInventATakeover(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	svc := newService(t, pool)
	ctx := context.Background()

	first, _ := svc.StartOrResume(ctx, w.assignment, w.student)
	for i := range 3 {
		if _, err := svc.StartOrResume(ctx, w.assignment, w.student); err != nil {
			t.Fatalf("reload %d: %v", i, err)
		}
	}

	for _, kind := range eventKinds(t, pool, first.Attempt.ID) {
		if kind == attempts.KindSessionTakeover {
			t.Fatal("a plain reload was reported to the teacher as a second device")
		}
	}
}

// A session that fell silent long ago is gone, however lively it once was.
func TestAnOldEventDoesNotKeepASessionLookingAlive(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	svc := newService(t, pool)
	ctx := context.Background()

	first, _ := svc.StartOrResume(ctx, w.assignment, w.student)
	if _, err := pool.Exec(ctx, `
		INSERT INTO app.attempt_events
		  (attempt_id, session_id, kind, occurred_at, received_at, client_seq)
		VALUES ($1::uuid,$2::uuid,'tab_hidden',$3,$3,0)`,
		first.Attempt.ID, first.SessionID, time.Now().Add(-30*time.Minute)); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.StartOrResume(ctx, w.assignment, w.student); err != nil {
		t.Fatalf("resume: %v", err)
	}
	for _, kind := range eventKinds(t, pool, first.Attempt.ID) {
		if kind == attempts.KindSessionTakeover {
			t.Fatal("a session silent for half an hour was treated as live")
		}
	}
}

// [40-open-items.md P3] Once started, deadline_at wins. A student mid-sentence
// when the assignment closes finishes their paper.
func TestAnAttemptInFlightSurvivesTheAssignmentClosing(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	svc := newService(t, pool)
	ctx := context.Background()

	first, err := svc.StartOrResume(ctx, w.assignment, w.student)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE app.assignments SET closed_at = now() - interval '1 minute' WHERE id = $1::uuid`,
		w.assignment); err != nil {
		t.Fatal(err)
	}

	resumed, err := svc.StartOrResume(ctx, w.assignment, w.student)
	if err != nil {
		t.Fatalf("a closed assignment took away an attempt already in flight: %v", err)
	}
	if resumed.Attempt.ID != first.Attempt.ID {
		t.Error("resumed a different attempt")
	}
}
