//go:build integration

package application_test

import (
	"context"
	"encoding/json"
	"math"
	"quizzivy/internal/core/adapters"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/modules/attempts/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/cqrs"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func setRelease(t *testing.T, pool *pgxpool.Pool, w world, release string, average bool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		UPDATE app.assignments SET review_release = $2, review_show_class_average = $3 WHERE id = $1::uuid`,
		w.assignment, release, average); err != nil {
		t.Fatal(err)
	}
}

func closeAssignment(t *testing.T, pool *pgxpool.Pool, w world) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`UPDATE app.assignments SET closed_at = now() - interval '1 second' WHERE id = $1::uuid`, w.assignment); err != nil {
		t.Fatal(err)
	}
}

type cohort struct {
	pool  *pgxpool.Pool
	w     world
	users []string
}

func newCohort(t *testing.T, pool *pgxpool.Pool, w world) *cohort {
	t.Helper()
	c := &cohort{pool: pool, w: w}
	t.Cleanup(func() {
		for _, q := range []string{
			`DELETE FROM app.attempts WHERE student_id = ANY($1::uuid[])`,
			`DELETE FROM app.class_members WHERE user_id = ANY($1::uuid[])`,
			`DELETE FROM app.users WHERE id = ANY($1::uuid[])`,
		} {
			if _, err := pool.Exec(context.Background(), q, c.users); err != nil {
				t.Errorf("cleaning up %q: %v", q, err)
			}
		}
	})
	return c
}

func (c *cohort) student(t *testing.T) string {
	t.Helper()
	var id string
	if err := c.pool.QueryRow(context.Background(),
		`INSERT INTO app.users (email, full_name, role_id)
		 VALUES ($1, 'Học viên lớp', (SELECT id FROM app.roles WHERE builtin_key = 'student')) RETURNING id::text`,
		"coh-"+nonce(t)+"@example.com").Scan(&id); err != nil {
		t.Fatal(err)
	}
	c.users = append(c.users, id)
	if _, err := c.pool.Exec(context.Background(),
		`INSERT INTO app.class_members (class_id, user_id, joined_via, added_by)
		 VALUES ($1::uuid, $2::uuid, 'admin', $3::uuid)`, c.w.class, id, c.w.admin); err != nil {
		t.Fatal(err)
	}
	return id
}

func (c *cohort) attempt(t *testing.T, student, status string, earned, total float64) {
	t.Helper()
	if _, err := c.pool.Exec(context.Background(), `
		INSERT INTO app.attempts
		  (assignment_id, test_version_id, student_id, attempt_no, status, session_id,
		   shuffle_seed, beacon_token_hash, started_at, deadline_at, submitted_at,
		   graded_at, void_reason, score_earned, score_total)
		VALUES ($1::uuid, $2::uuid, $3::uuid,
		        (SELECT coalesce(max(attempt_no), 0) + 1 FROM app.attempts
		          WHERE assignment_id = $1::uuid AND student_id = $3::uuid),
		        $4::app.attempt_status, gen_random_uuid(), 7, sha256('b'::bytea),
		        now() - interval '30 minutes', now() + interval '30 minutes', now(),
		        CASE WHEN $4 = 'graded' THEN now() END,
		        CASE WHEN $4 = 'voided' THEN 'reset by the teacher' END,
		        $5::numeric, $6::numeric)`,
		c.w.assignment, c.w.versionID, student, status, earned, total); err != nil {
		t.Fatal(err)
	}
}

func readAverage(t *testing.T, w world, read reader, attemptID string) *float64 {
	t.Helper()
	result, err := read.Result(context.Background(), attemptID, w.student)
	if err != nil {
		t.Fatal(err)
	}
	return result.ClassAverage
}

func within(got *float64, want float64) bool {
	return got != nil && math.Abs(*got-want) < 0.0005
}

type reader struct {
	handler cqrs.QueryHandler[query.Result, domain.Result]
}

func (r reader) Result(ctx context.Context, attemptID, studentID string) (domain.Result, error) {
	return r.handler.Handle(ctx, query.Result{AttemptID: attemptID, StudentID: studentID})
}

func TestTheClassAverageIsTheMeanOfEachStudentsBestGradedAttempt(t *testing.T) {
	pool := newPool(t)
	svc, w, session := submitted(t, pool)
	setRelease(t, pool, w, "on_submit", true)
	closeAssignment(t, pool, w)
	people := newCohort(t, pool, w)
	read := reader{svc.Queries.Result}

	first, second, third := people.student(t), people.student(t), people.student(t)
	people.attempt(t, first, "graded", 4, 10)
	people.attempt(t, first, "graded", 8, 10)
	people.attempt(t, second, "graded", 6, 10)
	people.attempt(t, second, "voided", 10, 10)
	people.attempt(t, third, "graded", 4, 10)
	people.attempt(t, third, "submitted", 10, 10)

	if got := readAverage(t, w, read, session.Attempt.ID); !within(got, 60) {
		t.Errorf("class average %v, want 60 (best of 80, 60 and 40 percent)", got)
	}
}

func TestTheClassAverageIsRoundedToTwoDecimals(t *testing.T) {
	pool := newPool(t)
	svc, w, session := submitted(t, pool)
	setRelease(t, pool, w, "on_submit", true)
	closeAssignment(t, pool, w)
	people := newCohort(t, pool, w)

	people.attempt(t, people.student(t), "graded", 1, 3)
	people.attempt(t, people.student(t), "graded", 1, 3)
	people.attempt(t, people.student(t), "graded", 2, 3)

	if got := readAverage(t, w, reader{svc.Queries.Result}, session.Attempt.ID); got == nil || *got != 44.44 {
		t.Errorf("class average %v, want 44.44", got)
	}
}

func TestTheClassAverageNeedsThreeStudentsWithAGradedAttempt(t *testing.T) {
	pool := newPool(t)
	svc, w, session := submitted(t, pool)
	setRelease(t, pool, w, "on_submit", true)
	closeAssignment(t, pool, w)
	people := newCohort(t, pool, w)
	read := reader{svc.Queries.Result}

	a, b := people.student(t), people.student(t)
	people.attempt(t, a, "graded", 9, 10)
	people.attempt(t, b, "graded", 5, 10)
	people.attempt(t, people.student(t), "voided", 10, 10)
	people.attempt(t, people.student(t), "submitted", 10, 10)
	people.attempt(t, people.student(t), "timed_out", 10, 10)
	_ = people.student(t)
	if got := readAverage(t, w, read, session.Attempt.ID); got != nil {
		t.Errorf("two graded students, and the voided, ungraded and absent ones, gave %v", *got)
	}

	c := people.student(t)
	people.attempt(t, c, "graded", 1, 10)
	if got := readAverage(t, w, read, session.Attempt.ID); !within(got, 50) {
		t.Errorf("three graded students gave %v, want 50", got)
	}

	if _, err := pool.Exec(context.Background(), `UPDATE app.users SET disabled_at = now() WHERE id = $1::uuid`, c); err != nil {
		t.Fatal(err)
	}
	if got := readAverage(t, w, read, session.Attempt.ID); got != nil {
		t.Errorf("a disabled student still counted: %v", *got)
	}
	if _, err := pool.Exec(context.Background(), `UPDATE app.users SET disabled_at = NULL WHERE id = $1::uuid`, c); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(),
		`DELETE FROM app.class_members WHERE class_id = $1::uuid AND user_id = $2::uuid`, w.class, c); err != nil {
		t.Fatal(err)
	}
	if got := readAverage(t, w, read, session.Attempt.ID); got != nil {
		t.Errorf("a student who left the class still counted: %v", *got)
	}
}

func TestTheClassAverageIsAbsentBeforeTheCloseAndWithTheSwitchOff(t *testing.T) {
	pool := newPool(t)
	svc, w, session := submitted(t, pool)
	people := newCohort(t, pool, w)
	read := reader{svc.Queries.Result}
	for range 3 {
		people.attempt(t, people.student(t), "graded", 5, 10)
	}

	setRelease(t, pool, w, "on_submit", true)
	if got := readAverage(t, w, read, session.Attempt.ID); got != nil {
		t.Errorf("an open assignment gave %v", *got)
	}
	closeAssignment(t, pool, w)
	setRelease(t, pool, w, "on_submit", false)
	if got := readAverage(t, w, read, session.Attempt.ID); got != nil {
		t.Errorf("a switch that is off gave %v", *got)
	}
	setRelease(t, pool, w, "on_submit", true)
	if got := readAverage(t, w, read, session.Attempt.ID); !within(got, 50) {
		t.Errorf("a closed assignment with the switch on gave %v, want 50", got)
	}
	setRelease(t, pool, w, "after_close", true)
	if got := readAverage(t, w, read, session.Attempt.ID); !within(got, 50) {
		t.Errorf("an after_close result, released, gave %v, want 50", got)
	}
}

func TestAnAfterCloseResultIsReleasedAtTheInstantOfTheClose(t *testing.T) {
	pool := newPool(t)
	_, w, session := submitted(t, pool)
	ctx := context.Background()
	setReview(t, pool, w, true, true, true)
	setRelease(t, pool, w, "after_close", true)
	people := newCohort(t, pool, w)
	for range 3 {
		people.attempt(t, people.student(t), "graded", 5, 10)
	}
	store := repositories.NewPostgres(db.NewContext(pool), adapters.AttemptStartGuard{})
	record, err := store.ByID(ctx, session.Attempt.ID, w.student)
	if err != nil {
		t.Fatal(err)
	}

	var closes time.Time
	if err := pool.QueryRow(ctx, `SELECT closes_at FROM app.assignments WHERE id = $1::uuid`, w.assignment).Scan(&closes); err != nil {
		t.Fatal(err)
	}
	load := func(now time.Time) domain.Result {
		t.Helper()
		result, err := store.LoadResult(ctx, record, now)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}

	before := load(closes.Add(-time.Microsecond))
	if before.Score != nil || before.Review.ShowScore || before.Review.ShowCorrectAnswers || before.Review.ShowExplanations {
		t.Errorf("a microsecond before the close the result shows: %+v %+v", before.Review, before.Score)
	}
	if before.ReleasesAt == nil || !before.ReleasesAt.Equal(closes) {
		t.Errorf("releases at %v, want %v", before.ReleasesAt, closes)
	}
	if before.ClassAverage != nil {
		t.Errorf("a withheld result carries the average %v", *before.ClassAverage)
	}
	if before.Review.Release != domain.ReleaseAfterClose || !before.Review.ShowClassAverage {
		t.Errorf("a withheld result lost the teacher's choices: %+v", before.Review)
	}

	at := load(closes)
	if at.Score == nil || !at.Review.ShowScore || !at.Review.ShowCorrectAnswers || !at.Review.ShowExplanations || at.ReleasesAt != nil {
		t.Errorf("the instant of the close the result is not released: %+v %+v %v", at.Review, at.Score, at.ReleasesAt)
	}
	if !within(at.ClassAverage, 50) {
		t.Errorf("the instant of the close the average is %v, want 50", at.ClassAverage)
	}

	early := closes.Add(-time.Hour)
	if _, err := pool.Exec(ctx, `UPDATE app.assignments SET closed_at = $2 WHERE id = $1::uuid`, w.assignment, early); err != nil {
		t.Fatal(err)
	}
	if got := load(early.Add(-time.Microsecond)); got.ReleasesAt == nil || !got.ReleasesAt.Equal(early) {
		t.Errorf("an early close: releases at %v, want %v", got.ReleasesAt, early)
	}
	if got := load(early); got.ReleasesAt != nil || got.Score == nil {
		t.Errorf("an early close, the instant it passes: %+v %v", got.Score, got.ReleasesAt)
	}

	late := closes.Add(time.Hour)
	if _, err := pool.Exec(ctx, `UPDATE app.assignments SET closed_at = $2 WHERE id = $1::uuid`, w.assignment, late); err != nil {
		t.Fatal(err)
	}
	if got := load(closes.Add(-time.Microsecond)); got.ReleasesAt == nil || !got.ReleasesAt.Equal(closes) {
		t.Errorf("a close recorded after the window ended: releases at %v, want the window's end %v", got.ReleasesAt, closes)
	}
	if got := load(closes); got.ReleasesAt != nil {
		t.Errorf("a close recorded after the window ended held the result past it: %v", got.ReleasesAt)
	}
}

func TestAWithheldResultNeverSelectsTheKeyTheExplanationsOrTheGradersWords(t *testing.T) {
	pool := newPool(t)
	svc, w, session := submitted(t, pool)
	ctx := context.Background()
	setReview(t, pool, w, true, true, true)
	setRelease(t, pool, w, "after_close", false)
	const comment = "NHAN-XET-CUA-GIAO-VIEN"
	if _, err := pool.Exec(ctx, `
		UPDATE app.attempt_answers SET manual_score = 3, graded_at = now(), grader_comment = $3
		 WHERE attempt_id = $1::uuid AND question_id = $2::uuid`, session.Attempt.ID, w.essay, comment); err != nil {
		t.Fatal(err)
	}
	read := reader{svc.Queries.Result}

	held, err := read.Result(ctx, session.Attempt.ID, w.student)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(held)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{secretExplanation, secretBlankAnswer, secretSampleAnswer, comment} {
		if strings.Contains(string(raw), secret) {
			t.Errorf("a withheld result carries %q", secret)
		}
	}
	for _, q := range held.Questions {
		if q.Earned != nil || q.CorrectOptions != nil || q.CorrectAnswers != nil || q.Explanation != nil || q.GraderComment != nil {
			t.Errorf("a withheld question carries more than the student's own answer: %+v", q)
		}
	}
	if held.Questions[0].Answer == nil && held.Questions[1].Answer == nil {
		t.Error("a withheld result hides the student's own answers")
	}

	closeAssignment(t, pool, w)
	freed, err := read.Result(ctx, session.Attempt.ID, w.student)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = json.Marshal(freed)
	if err != nil {
		t.Fatal(err)
	}
	for _, shown := range []string{secretExplanation, secretBlankAnswer, comment} {
		if !strings.Contains(string(raw), shown) {
			t.Errorf("a released result lacks %q", shown)
		}
	}
	if strings.Contains(string(raw), secretSampleAnswer) {
		t.Error("a released result carries the sample answer")
	}
}
