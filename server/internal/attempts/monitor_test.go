package attempts_test

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/attempts"
)

type queryCounter struct{ n atomic.Int32 }

func (c *queryCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	c.n.Add(1)
	return ctx
}

func (c *queryCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func countedPool(t *testing.T) (*pgxpool.Pool, *queryCounter) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	counter := &queryCounter{}
	cfg.ConnConfig.Tracer = counter
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool, counter
}

func enrol(t *testing.T, pool *pgxpool.Pool, w world, n int) []string {
	t.Helper()
	ctx := context.Background()
	id := nonce(t)
	ids := make([]string, n)
	for i := range n {
		if err := pool.QueryRow(ctx,
			`INSERT INTO app.users (email, full_name, role) VALUES ($1, $2, 'student') RETURNING id::text`,
			"mon-"+id+"-"+string(rune('a'+i%26))+string(rune('a'+i/26))+"@example.com",
			"Học viên "+string(rune('A'+i%26))+string(rune('a'+i/26))).Scan(&ids[i]); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO app.class_members (class_id, user_id, joined_via, added_by)
			VALUES ($1::uuid, $2::uuid, 'admin', $3::uuid)`, w.class, ids[i], w.admin); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM app.attempt_answers WHERE attempt_id IN
			(SELECT id FROM app.attempts WHERE student_id = ANY($1::uuid[]))`, ids)
		_, _ = pool.Exec(c, `DELETE FROM app.attempt_events WHERE attempt_id IN
			(SELECT id FROM app.attempts WHERE student_id = ANY($1::uuid[]))`, ids)
		_, _ = pool.Exec(c, `DELETE FROM app.attempts WHERE student_id = ANY($1::uuid[])`, ids)
		_, _ = pool.Exec(c, `DELETE FROM app.class_members WHERE user_id = ANY($1::uuid[])`, ids)
		_, _ = pool.Exec(c, `DELETE FROM app.users WHERE id = ANY($1::uuid[])`, ids)
	})
	return ids
}

func handIn(t *testing.T, pool *pgxpool.Pool, w world, studentID string, no int, status string) string {
	t.Helper()
	var id string
	submitted := "now()"
	if status == "in_progress" {
		submitted = "NULL"
	}
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO app.attempts
		  (assignment_id, test_version_id, student_id, attempt_no, status, session_id,
		   shuffle_seed, beacon_token_hash, started_at, deadline_at, submitted_at,
		   score_earned, score_total, void_reason)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5::app.attempt_status, gen_random_uuid(),
		        7, sha256('x'::bytea), now() - interval '20 minutes', now() + interval '40 minutes',
		        `+submitted+`,
		        CASE WHEN $5 IN ('submitted','graded','timed_out') THEN 6.5 END,
		        CASE WHEN $5 IN ('submitted','graded','timed_out') THEN 10 END,
		        CASE WHEN $5 = 'voided' THEN 'test' END)
		RETURNING id::text`, w.assignment, w.versionID, studentID, no, status).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestTheMonitorIsTwoQueriesForFiftyStudentsAndThirtyAttempts(t *testing.T) {
	pool, counter := countedPool(t)
	w := seedWorld(t, pool, openAssignment())
	students := enrol(t, pool, w, 49)
	for i, id := range students[:30] {
		status := "submitted"
		if i%3 == 0 {
			status = "in_progress"
		}
		handIn(t, pool, w, id, 1, status)
	}

	store := attempts.NewStore(pool)
	counter.n.Store(0)
	monitor, err := store.Monitor(context.Background(), w.assignment, time.Now())
	if err != nil {
		t.Fatalf("monitor: %v", err)
	}
	if got := counter.n.Load(); got != 2 {
		t.Errorf("%d queries for the monitor, want exactly 2 (§13.8)", got)
	}
	if len(monitor.Rows) != 50 {
		t.Fatalf("%d rows, want 50: the roster, not the attempts table, is the left side", len(monitor.Rows))
	}
	if monitor.QuestionCount != 4 {
		t.Errorf("questionCount %d, want the fixture's 4", monitor.QuestionCount)
	}

	states := map[string]int{}
	for _, r := range monitor.Rows {
		states[r.State]++
	}
	if states["not_started"] != 20 || states["in_progress"] != 10 || states["submitted"] != 20 {
		t.Errorf("states %v", states)
	}
	// Decisions float up: everyone in progress before anyone who has not started.
	for i, r := range monitor.Rows {
		if i < 10 && r.State != "in_progress" {
			t.Fatalf("row %d is %s; the in-progress rows come first", i, r.State)
		}
		if i >= 10 && i < 30 && r.State != "not_started" {
			t.Fatalf("row %d is %s; not-started rows come next", i, r.State)
		}
	}
}

func TestARowShowsTheAttemptThatStillCountsAndReadsProgressFromTheAnswers(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	answerEverythingRight(t, pool, w, session, svc)

	other := enrol(t, pool, w, 1)[0]
	handIn(t, pool, w, other, 1, "voided")
	live := handIn(t, pool, w, other, 2, "submitted")

	monitor, err := svc.Monitor(context.Background(), w.assignment)
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]attempts.MonitorRow{}
	for _, r := range monitor.Rows {
		rows[r.StudentID] = r
	}

	mine := rows[w.student]
	if mine.State != "in_progress" || mine.AnsweredCount == nil || *mine.AnsweredCount != 4 {
		t.Errorf("my row %+v, want in_progress with 4 answers", mine)
	}
	if mine.Score != nil {
		t.Errorf("an attempt in progress has no score yet, got %+v", *mine.Score)
	}

	theirs := rows[other]
	if theirs.AttemptID == nil || *theirs.AttemptID != live || theirs.State != "submitted" {
		t.Errorf("the second, live attempt should stand for the student, got %+v", theirs)
	}
	if theirs.Score == nil || theirs.Score.Earned != 6.5 || theirs.Score.Total != 10 {
		t.Errorf("score %+v, want 6.5/10", theirs.Score)
	}
}

func TestTheMonitorClosesAnAttemptWhoseTimeRanOutBeforeReporting(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	if _, err := pool.Exec(context.Background(),
		`UPDATE app.attempts SET started_at = now() - interval '2 minutes', deadline_at = now() - interval '1 second' WHERE id = $1::uuid`,
		session.Attempt.ID); err != nil {
		t.Fatal(err)
	}

	monitor, err := svc.Monitor(context.Background(), w.assignment)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range monitor.Rows {
		if r.StudentID == w.student && r.State != "timed_out" {
			t.Errorf("state %s, want timed_out: the monitor never shows a live row past its deadline", r.State)
		}
	}
}

func TestAnUnknownAssignmentIsNotAnEmptyMonitor(t *testing.T) {
	pool := newPool(t)
	_, err := attempts.NewStore(pool).Monitor(context.Background(), "01935000-0000-7000-8000-00000000dead", time.Now())
	if err != attempts.ErrNotFound {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}
