package students_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/students"
)

// world is one admin, one class, one published test with two versions, and a
// student whose attempts the tests then arrange.
type world struct {
	admin   string
	class   string
	student string
	testID  string
	version string
	points  string
}

func seedWorld(t *testing.T, pool *pgxpool.Pool, totalPoints string) world {
	t.Helper()
	ctx := context.Background()
	id := nonce(t)
	var w world
	w.points = totalPoints

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(pool.QueryRow(ctx,
		`INSERT INTO app.users (email, full_name, role) VALUES ($1,'GV','admin') RETURNING id::text`,
		"stat-a-"+id+"@example.com").Scan(&w.admin))
	must(pool.QueryRow(ctx,
		`INSERT INTO app.users (email, full_name, role) VALUES ($1,'Học Viên Thống Kê','student') RETURNING id::text`,
		"stat-s-"+id+"@example.com").Scan(&w.student))
	must(pool.QueryRow(ctx,
		`INSERT INTO app.classes (name) VALUES ($1) RETURNING id::text`, "Lop "+id).Scan(&w.class))
	must(func() error {
		_, err := pool.Exec(ctx,
			`INSERT INTO app.class_members (class_id, user_id, joined_via, added_by)
			 VALUES ($1::uuid,$2::uuid,'admin',$3::uuid)`, w.class, w.student, w.admin)
		return err
	}())
	must(pool.QueryRow(ctx,
		`INSERT INTO app.tests (title, status, current_version, created_by)
		 VALUES ($1,'published',1,$2::uuid) RETURNING id::text`,
		"De "+id, w.admin).Scan(&w.testID))
	must(pool.QueryRow(ctx,
		`INSERT INTO app.test_versions (test_id, version, total_points, published_by)
		 VALUES ($1::uuid,1,$2,$3::uuid) RETURNING id::text`,
		w.testID, totalPoints, w.admin).Scan(&w.version))

	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM app.attempt_answers WHERE attempt_id IN
			(SELECT id FROM app.attempts WHERE student_id = $1::uuid)`, w.student)
		_, _ = pool.Exec(c, `DELETE FROM app.attempts WHERE student_id = $1::uuid`, w.student)
		_, _ = pool.Exec(c, `DELETE FROM app.assignments WHERE test_id = $1::uuid`, w.testID)
		_, _ = pool.Exec(c, `DELETE FROM app.test_versions WHERE test_id = $1::uuid`, w.testID)
		_, _ = pool.Exec(c, `DELETE FROM app.tests WHERE id = $1::uuid`, w.testID)
		_, _ = pool.Exec(c, `DELETE FROM app.class_members WHERE class_id = $1::uuid`, w.class)
		_, _ = pool.Exec(c, `DELETE FROM app.classes WHERE id = $1::uuid`, w.class)
		_, _ = pool.Exec(c, `DELETE FROM app.audit_log WHERE actor_user_id IN ($1::uuid,$2::uuid)`, w.admin, w.student)
		_, _ = pool.Exec(c, `DELETE FROM app.users WHERE id IN ($1::uuid,$2::uuid)`, w.admin, w.student)
	})
	return w
}

func (w world) assignment(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO app.assignments
		       (test_id, test_version_id, opens_at, closes_at, duration_minutes, created_by)
		VALUES ($1::uuid,$2::uuid, now() - interval '2 hours', now() + interval '2 hours', 45, $3::uuid)
		RETURNING id::text`, w.testID, w.version, w.admin).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

type attempt struct {
	assignment string
	no         int
	status     string
	earned     *string
	total      *string
	flagged    bool
	live       bool
}

func (w world) attempt(t *testing.T, pool *pgxpool.Pool, a attempt) string {
	t.Helper()
	deadline := "now() + interval '1 hour'"
	if !a.live {
		deadline = "now() - interval '1 hour'"
	}
	graded := "NULL"
	if a.status == "graded" {
		graded = "now()"
	}
	submitted := "now() - interval '30 minutes'"
	if a.status == "in_progress" {
		submitted = "NULL"
	}
	void := "NULL"
	if a.status == "voided" {
		void = "'test'"
	}

	var id string
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO app.attempts
		       (assignment_id, test_version_id, student_id, attempt_no, status,
		        session_id, shuffle_seed, beacon_token_hash,
		        started_at, deadline_at, submitted_at, graded_at, void_reason,
		        score_earned, score_total, flagged)
		VALUES ($1::uuid,$2::uuid,$3::uuid,$4,$5::app.attempt_status,
		        gen_random_uuid(), 1, sha256('b'::bytea),
		        now() - interval '90 minutes', `+deadline+`, `+submitted+`,
		        `+graded+`, `+void+`, $6::numeric, $7::numeric, $8)
		RETURNING id::text`,
		a.assignment, w.version, w.student, a.no, a.status,
		a.earned, a.total, a.flagged).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func statsOf(t *testing.T, store *students.Store, id string) students.Stats {
	t.Helper()
	got, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	return got.Stats
}

func p(v string) *string { return &v }

// show renders a nullable score for a failure message; %v on the pointer prints
// an address, which says nothing about what went wrong.
func show(v *float64) string {
	if v == nil {
		return "<none>"
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

// The rule §7's maxAttempts implies: "lấy điểm lượt cao nhất".
func TestTheAverageTakesTheBestAttemptPerAssignment(t *testing.T) {
	pool := newPool(t)
	store := students.NewStore(pool)
	w := seedWorld(t, pool, "10.00")
	a := w.assignment(t, pool)

	w.attempt(t, pool, attempt{assignment: a, no: 1, status: "graded", earned: p("4.00"), total: p("10.00")})
	w.attempt(t, pool, attempt{assignment: a, no: 2, status: "graded", earned: p("9.00"), total: p("10.00")})

	stats := statsOf(t, store, w.student)
	if stats.ScoreEarned == nil || *stats.ScoreEarned != 9 {
		t.Errorf("earned = %s, want 9 (the better attempt, not the sum or the latest)",
			show(stats.ScoreEarned))
	}
	if stats.ScoreTotal == nil || *stats.ScoreTotal != 10 {
		t.Errorf("total = %s, want 10 (one assignment counted once)", show(stats.ScoreTotal))
	}
	// Two attempts, one assignment.
	if stats.SubmittedCount != 1 {
		t.Errorf("submittedCount = %d, want 1: it counts assignments, not attempts", stats.SubmittedCount)
	}
}

// The deck's own gradebook proves this arithmetically: a column with three
// scores and one "chưa nộp" averages over three, not four.
func TestAnUnsubmittedAssignmentIsAbsentRatherThanZero(t *testing.T) {
	pool := newPool(t)
	store := students.NewStore(pool)
	w := seedWorld(t, pool, "10.00")

	done := w.assignment(t, pool)
	w.attempt(t, pool, attempt{assignment: done, no: 1, status: "graded", earned: p("8.00"), total: p("10.00")})
	w.assignment(t, pool) // assigned, never started

	stats := statsOf(t, store, w.student)
	if *stats.ScoreEarned != 8 || *stats.ScoreTotal != 10 {
		t.Errorf("got %v/%v, want 8/10 — the untouched assignment must not enter either sum",
			*stats.ScoreEarned, *stats.ScoreTotal)
	}
}

// attempt_answers.final_score is NULL while a short answer waits for a human,
// and sum() skips NULLs — so counting an ungraded attempt scores an unread
// essay as nought.
func TestASubmittedButUngradedAttemptDoesNotEnterTheAverage(t *testing.T) {
	pool := newPool(t)
	store := students.NewStore(pool)
	w := seedWorld(t, pool, "10.00")

	graded := w.assignment(t, pool)
	w.attempt(t, pool, attempt{assignment: graded, no: 1, status: "graded", earned: p("7.00"), total: p("10.00")})
	waiting := w.assignment(t, pool)
	w.attempt(t, pool, attempt{assignment: waiting, no: 1, status: "submitted"})

	stats := statsOf(t, store, w.student)
	if *stats.ScoreEarned != 7 || *stats.ScoreTotal != 10 {
		t.Errorf("got %v/%v, want 7/10", *stats.ScoreEarned, *stats.ScoreTotal)
	}
	// It still counts as work that reached the teacher.
	if stats.SubmittedCount != 2 {
		t.Errorf("submittedCount = %d, want 2", stats.SubmittedCount)
	}
}

func TestNothingGradedReadsAsNoScoreRatherThanZero(t *testing.T) {
	pool := newPool(t)
	store := students.NewStore(pool)
	w := seedWorld(t, pool, "10.00")
	a := w.assignment(t, pool)
	w.attempt(t, pool, attempt{assignment: a, no: 1, status: "submitted"})

	stats := statsOf(t, store, w.student)
	if stats.ScoreEarned != nil || stats.ScoreTotal != nil {
		t.Errorf("got %s/%s, want no score at all",
			show(stats.ScoreEarned), show(stats.ScoreTotal))
	}
}

// Voiding is an administrative erasure; counting it would be a silent grade
// change.
func TestAVoidedAttemptIsExcludedEverywhere(t *testing.T) {
	pool := newPool(t)
	store := students.NewStore(pool)
	w := seedWorld(t, pool, "10.00")
	a := w.assignment(t, pool)
	w.attempt(t, pool, attempt{assignment: a, no: 1, status: "voided", earned: p("2.00"), total: p("10.00"), flagged: true})

	stats := statsOf(t, store, w.student)
	if stats.ScoreEarned != nil {
		t.Errorf("a voided attempt entered the average: %v", *stats.ScoreEarned)
	}
	if stats.SubmittedCount != 0 {
		t.Errorf("submittedCount = %d, want 0", stats.SubmittedCount)
	}
	if stats.FlaggedCount != 0 {
		t.Errorf("flaggedCount = %d, want 0", stats.FlaggedCount)
	}
}

// Nothing in this system flips a stale in-progress attempt: assignment status
// is derived precisely so there is no scheduler (D-18).
func TestAnExpiredInProgressAttemptIsNotStillTakingIt(t *testing.T) {
	pool := newPool(t)
	store := students.NewStore(pool)
	w := seedWorld(t, pool, "10.00")

	past := w.assignment(t, pool)
	w.attempt(t, pool, attempt{assignment: past, no: 1, status: "in_progress", live: false})
	if statsOf(t, store, w.student).LiveAttempt {
		t.Error(`an attempt past its deadline still reads "đang làm bài"`)
	}

	now := w.assignment(t, pool)
	w.attempt(t, pool, attempt{assignment: now, no: 1, status: "in_progress", live: true})
	if !statsOf(t, store, w.student).LiveAttempt {
		t.Error("a live attempt inside its deadline is not reported")
	}
}

func TestTheRosterAndFlagsComeBackWithTheRow(t *testing.T) {
	pool := newPool(t)
	store := students.NewStore(pool)
	w := seedWorld(t, pool, "10.00")
	a := w.assignment(t, pool)
	w.attempt(t, pool, attempt{assignment: a, no: 1, status: "graded", earned: p("5.00"), total: p("10.00"), flagged: true})

	got, err := store.Get(context.Background(), w.student)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Classes) != 1 || got.Classes[0].ID != w.class {
		t.Fatalf("classes = %+v", got.Classes)
	}
	if got.Classes[0].JoinedVia != "admin" {
		t.Errorf("joinedVia = %q, want admin", got.Classes[0].JoinedVia)
	}
	if got.Stats.FlaggedCount != 1 {
		t.Errorf("flaggedCount = %d, want 1", got.Stats.FlaggedCount)
	}
}

// The whole reason the stats are not on User: an admin id in the URL must not
// reach these paths at all.
func TestAnAdminIsNotAStudent(t *testing.T) {
	pool := newPool(t)
	store := students.NewStore(pool)
	w := seedWorld(t, pool, "10.00")

	if _, err := store.Get(context.Background(), w.admin); err == nil {
		t.Fatal("Get returned an admin account")
	}
	if err := store.ResetPassword(context.Background(),
		students.Request{ActorID: w.admin}, w.admin, "$argon2id$fake", time.Now()); err == nil {
		t.Fatal("ResetPassword accepted an admin id: that is account takeover")
	}
}
