package dashboard_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/dashboard"
)

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func nonce(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

// fixture builds one published test, one assignment and one submitted attempt
// carrying a short answer nobody has graded.
type fixture struct {
	assignment string
	attempt    string
	student    string
}

func seed(t *testing.T, pool *pgxpool.Pool, opensAt, closesAt time.Time, flagged bool) fixture {
	t.Helper()
	ctx := context.Background()
	id := nonce(t)

	var author, student string
	if err := pool.QueryRow(ctx,
		`INSERT INTO app.users (email, full_name, role) VALUES ($1,'Giáo viên','admin') RETURNING id::text`,
		"dash-a-"+id+"@example.com").Scan(&author); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO app.users (email, full_name, role) VALUES ($1,'Học viên','student') RETURNING id::text`,
		"dash-s-"+id+"@example.com").Scan(&student); err != nil {
		t.Fatal(err)
	}

	var testID, versionID, sectionID, questionID, assignmentID, attemptID string
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(pool.QueryRow(ctx,
		`INSERT INTO app.tests (title, status, current_version, created_by)
		 VALUES ('Dashboard fixture','published',1,$1) RETURNING id::text`, author).Scan(&testID))
	must(pool.QueryRow(ctx,
		`INSERT INTO app.test_versions (test_id, version, total_points, published_by)
		 VALUES ($1,1,'2.00',$2) RETURNING id::text`, testID, author).Scan(&versionID))
	must(pool.QueryRow(ctx,
		`INSERT INTO app.test_version_sections (test_version_id, ordinal, title)
		 VALUES ($1,0,'Phần 1') RETURNING id::text`, versionID).Scan(&sectionID))
	must(pool.QueryRow(ctx,
		`INSERT INTO app.test_version_questions
		        (test_version_section_id, ordinal, type, prompt, points)
		 VALUES ($1,0,'short_answer','Viết một câu','2.00') RETURNING id::text`,
		sectionID).Scan(&questionID))
	must(pool.QueryRow(ctx,
		`INSERT INTO app.assignments
		        (test_id, test_version_id, opens_at, closes_at, duration_minutes, created_by,
		         published_at)
		 VALUES ($1,$2,$3,$4,45,$5, now()) RETURNING id::text`,
		testID, versionID, opensAt, closesAt, author).Scan(&assignmentID))
	must(pool.QueryRow(ctx,
		`INSERT INTO app.attempts
		        (assignment_id, test_version_id, student_id, attempt_no, status,
		         session_id, shuffle_seed, beacon_token_hash, deadline_at,
		         submitted_at, flagged)
		 VALUES ($1,$2,$3,1,'submitted', gen_random_uuid(), 42,
		         sha256('beacon'::bytea), now() + interval '1 hour', now(), $4)
		 RETURNING id::text`,
		assignmentID, versionID, student, flagged).Scan(&attemptID))
	must(func() error {
		_, err := pool.Exec(ctx,
			`INSERT INTO app.attempt_answers (attempt_id, question_id, payload, requires_manual)
			 VALUES ($1,$2,'{"text":"xin chào"}'::jsonb, true)`, attemptID, questionID)
		return err
	}())

	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM app.attempt_answers WHERE attempt_id = $1`, attemptID)
		_, _ = pool.Exec(c, `DELETE FROM app.attempts WHERE id = $1`, attemptID)
		_, _ = pool.Exec(c, `DELETE FROM app.assignments WHERE id = $1`, assignmentID)
		_, _ = pool.Exec(c, `DELETE FROM app.test_version_questions WHERE test_version_section_id = $1`, sectionID)
		_, _ = pool.Exec(c, `DELETE FROM app.test_version_sections WHERE id = $1`, sectionID)
		_, _ = pool.Exec(c, `DELETE FROM app.test_versions WHERE id = $1`, versionID)
		_, _ = pool.Exec(c, `DELETE FROM app.audit_log WHERE actor_user_id IN ($1,$2)`, author, student)
		_, _ = pool.Exec(c, `DELETE FROM app.tests WHERE id = $1`, testID)
		_, _ = pool.Exec(c, `DELETE FROM app.users WHERE id IN ($1,$2)`, author, student)
	})

	return fixture{assignment: assignmentID, attempt: attemptID, student: student}
}

func TestAnOpenAssignmentIsCountedAndAClosedOneIsNot(t *testing.T) {
	pool := newPool(t)
	store := dashboard.NewStore(pool)
	ctx := context.Background()

	before, err := store.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Open now.
	seed(t, pool, time.Now().Add(-time.Hour), time.Now().Add(time.Hour), false)
	// Already finished: outside the window, so not "open".
	seed(t, pool, time.Now().Add(-48*time.Hour), time.Now().Add(-24*time.Hour), false)

	after, err := store.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := after.OpenAssignments - before.OpenAssignments; got != 1 {
		t.Errorf("open assignments: want +1, got +%d", got)
	}
}

func TestAnUngradedShortAnswerIsTheGradingQueue(t *testing.T) {
	pool := newPool(t)
	store := dashboard.NewStore(pool)
	ctx := context.Background()

	before, err := store.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	f := seed(t, pool, time.Now().Add(-time.Hour), time.Now().Add(time.Hour), false)

	after, err := store.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := after.AwaitingGrading - before.AwaitingGrading; got != 1 {
		t.Fatalf("awaiting grading: want +1, got +%d", got)
	}

	// Grading it empties the queue -- the count is about work left, not work done.
	if _, err := pool.Exec(ctx,
		`UPDATE app.attempt_answers SET manual_score = '2.00', graded_at = now()
		  WHERE attempt_id = $1`, f.attempt); err != nil {
		t.Fatal(err)
	}
	graded, err := store.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := graded.AwaitingGrading - before.AwaitingGrading; got != 0 {
		t.Errorf("after grading: want +0, got +%d", got)
	}
}

func TestAFlaggedAttemptIsCountedAndAppearsInRecent(t *testing.T) {
	pool := newPool(t)
	store := dashboard.NewStore(pool)
	ctx := context.Background()

	before, err := store.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	f := seed(t, pool, time.Now().Add(-time.Hour), time.Now().Add(time.Hour), true)

	after, err := store.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := after.FlaggedAttempts - before.FlaggedAttempts; got != 1 {
		t.Errorf("flagged: want +1, got +%d", got)
	}

	var found *dashboard.Recent
	for i := range after.Recent {
		if after.Recent[i].ID == f.attempt {
			found = &after.Recent[i]
		}
	}
	if found == nil {
		t.Fatal("the attempt just created is not in the recent list")
	}
	if found.StudentName != "Học viên" || found.TestTitle != "Dashboard fixture" {
		t.Errorf("recent row is not resolved: %+v", *found)
	}
	if found.PendingManual != 1 {
		t.Errorf("pendingManual: want 1, got %d", found.PendingManual)
	}
}

func TestActiveStudentsCountsDistinctRecentSitters(t *testing.T) {
	pool := newPool(t)
	store := dashboard.NewStore(pool)
	ctx := context.Background()

	before, err := store.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	f := seed(t, pool, time.Now().Add(-time.Hour), time.Now().Add(time.Hour), false)

	after, err := store.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := after.ActiveStudents - before.ActiveStudents; got != 1 {
		t.Errorf("active students: want +1, got +%d", got)
	}

	// Older than the window: the dashboard is today's queue, not all history.
	if _, err := pool.Exec(ctx,
		`UPDATE app.attempts SET started_at = now() - interval '30 days' WHERE id = $1`,
		f.attempt); err != nil {
		t.Fatal(err)
	}
	stale, err := store.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := stale.ActiveStudents - before.ActiveStudents; got != 0 {
		t.Errorf("after ageing out: want +0, got +%d", got)
	}
}
