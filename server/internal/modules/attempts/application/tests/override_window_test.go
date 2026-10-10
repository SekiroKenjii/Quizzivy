//go:build integration

package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/core/adapters"
	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/modules/attempts/repositories"
	"quizzivy/internal/platform/db"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func giveOverride(t *testing.T, pool *pgxpool.Pool, w world, student string, closesAt *time.Time, minutes *int, extra int) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO app.assignment_student_overrides
		       (assignment_id, student_id, closes_at, duration_minutes, extra_attempts, reason, created_by)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, 'Kiểm thử ngoại lệ', $6::uuid)`,
		w.assignment, student, closesAt, minutes, extra, w.admin); err != nil {
		t.Fatalf("giving the override: %v", err)
	}
}

func later(offset time.Duration) *time.Time {
	at := time.Now().Add(offset).Truncate(time.Second)
	return &at
}

func overrideInt(n int) *int { return &n }

func TestAnOverrideGivesTheStudentTheirOwnTimeLimit(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	svc := newService(t, pool)
	classmate := newCohort(t, pool, w).student(t)
	giveOverride(t, pool, w, w.student, nil, overrideInt(120), 0)
	ctx := context.Background()

	own, err := svc.Commands.StartOrResume.Handle(ctx, command.StartOrResume{AssignmentID: w.assignment, StudentID: w.student})
	if err != nil {
		t.Fatalf("start with an override: %v", err)
	}
	plain, err := svc.Commands.StartOrResume.Handle(ctx, command.StartOrResume{AssignmentID: w.assignment, StudentID: classmate})
	if err != nil {
		t.Fatalf("start without one: %v", err)
	}
	if d := own.Attempt.DeadlineAt.Sub(own.Attempt.StartedAt); d < 119*time.Minute || d > 121*time.Minute {
		t.Errorf("the student with 120 minutes has %v, want about 120m", d)
	}
	if d := plain.Attempt.DeadlineAt.Sub(plain.Attempt.StartedAt); d < 59*time.Minute || d > 61*time.Minute {
		t.Errorf("the classmate has %v, want the assignment's 60m", d)
	}
}

func TestTheStudentsOwnCloseCutsTheStudentsOwnDeadline(t *testing.T) {
	pool := newPool(t)
	o := openAssignment()
	o.opensAt, o.closesAt = time.Now().Add(-3*time.Hour), time.Now().Add(-time.Hour)
	w := seedWorld(t, pool, o)
	giveOverride(t, pool, w, w.student, later(45*time.Minute), overrideInt(60), 0)

	got, err := newService(t, pool).Commands.StartOrResume.Handle(context.Background(), command.StartOrResume{AssignmentID: w.assignment, StudentID: w.student})
	if err != nil {
		t.Fatalf("start under an override: %v", err)
	}
	if d := got.Attempt.DeadlineAt.Sub(got.Attempt.StartedAt); d < 44*time.Minute || d > 46*time.Minute {
		t.Errorf("the deadline is %v after the start, want the student's 45 minutes to their own close", d)
	}
}

func TestAClosedAssignmentOpensOnlyForAStudentWhoseOverrideIsStillAhead(t *testing.T) {
	for name, shape := range map[string]func(*worldOpts){
		"its window ended": func(o *worldOpts) {
			o.opensAt, o.closesAt = time.Now().Add(-3*time.Hour), time.Now().Add(-time.Hour)
		},
		"it was closed early": func(o *worldOpts) {
			early := time.Now().Add(-time.Minute)
			o.closedAt = &early
		},
	} {
		t.Run(name, func(t *testing.T) {
			pool := newPool(t)
			o := openAssignment()
			shape(&o)
			w := seedWorld(t, pool, o)
			people := newCohort(t, pool, w)
			svc := newService(t, pool)
			ctx := context.Background()
			reopened, expired, plain := w.student, people.student(t), people.student(t)
			giveOverride(t, pool, w, reopened, later(time.Hour), nil, 0)
			giveOverride(t, pool, w, expired, later(-time.Minute), nil, 1)

			if _, err := svc.Commands.StartOrResume.Handle(ctx, command.StartOrResume{AssignmentID: w.assignment, StudentID: reopened}); err != nil {
				t.Errorf("the student with an override ahead: %v", err)
			}
			for who, student := range map[string]string{"a classmate without one": plain, "a student whose override has passed": expired} {
				if _, err := svc.Commands.StartOrResume.Handle(ctx, command.StartOrResume{AssignmentID: w.assignment, StudentID: student}); !errors.Is(err, domain.ErrAssignmentClosed) {
					t.Errorf("%s: %v, want ErrAssignmentClosed", who, err)
				}
			}
		})
	}
}

func TestExtraAttemptsRaiseTheLimitForThatStudentAlone(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	svc := newService(t, pool)
	classmate := newCohort(t, pool, w).student(t)
	giveOverride(t, pool, w, w.student, nil, nil, 1)
	ctx := context.Background()

	sit := func(student string, wantNo, wantRemaining int) domain.Session {
		t.Helper()
		session, err := svc.Commands.StartOrResume.Handle(ctx, command.StartOrResume{AssignmentID: w.assignment, StudentID: student})
		if err != nil {
			t.Fatalf("attempt %d: %v", wantNo, err)
		}
		if session.Attempt.AttemptNo != wantNo || session.RemainingAttempts != wantRemaining {
			t.Errorf("attempt %d, %d remaining; want attempt %d, %d remaining", session.Attempt.AttemptNo, session.RemainingAttempts, wantNo, wantRemaining)
		}
		if _, err := svc.Commands.Submit.Handle(ctx, command.Submit{AttemptID: session.Attempt.ID, StudentID: student, Reason: domain.Manual}); err != nil {
			t.Fatal(err)
		}
		return session
	}
	sit(w.student, 1, 1)
	second := sit(w.student, 2, 0)
	if _, err := svc.Commands.StartOrResume.Handle(ctx, command.StartOrResume{AssignmentID: w.assignment, StudentID: w.student}); !errors.Is(err, domain.ErrLimitReached) {
		t.Errorf("a third attempt: %v, want ErrLimitReached", err)
	}

	sit(classmate, 1, 0)
	if _, err := svc.Commands.StartOrResume.Handle(ctx, command.StartOrResume{AssignmentID: w.assignment, StudentID: classmate}); !errors.Is(err, domain.ErrLimitReached) {
		t.Errorf("a classmate's second attempt: %v, want ErrLimitReached", err)
	}

	result, err := svc.Queries.Result.Handle(ctx, query.Result{AttemptID: second.Attempt.ID, StudentID: w.student})
	if err != nil || result.MaxAttempts != 2 {
		t.Errorf("the result of attempt 2 says %d attempts (%v), want 2", result.MaxAttempts, err)
	}
	reviews := repositories.NewReviews(db.NewContext(pool))
	review, err := reviews.Get(ctx, everyone, second.Attempt.ID)
	if err != nil || review.MaxAttempts != 2 {
		t.Errorf("the teacher's review of attempt 2 says %d attempts (%v), want 2", review.MaxAttempts, err)
	}
}

func TestAReloadedAttemptCountsTheStudentsOwnAttempts(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	svc := newService(t, pool)
	ctx := context.Background()
	giveOverride(t, pool, w, w.student, nil, nil, 2)

	live, err := svc.Commands.StartOrResume.Handle(ctx, command.StartOrResume{AssignmentID: w.assignment, StudentID: w.student})
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := svc.Queries.Get.Handle(ctx, query.Get{AttemptID: live.Attempt.ID, StudentID: w.student})
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.RemainingAttempts != 2 || live.RemainingAttempts != 2 {
		t.Errorf("remaining after the start %d and after a reload %d, want 2 and 2 of 3", live.RemainingAttempts, reloaded.RemainingAttempts)
	}
}

func TestTheReadersCloseHoldsTheirResultAndTheAssignmentsCloseHoldsTheAverage(t *testing.T) {
	pool := newPool(t)
	_, w, session := submitted(t, pool)
	ctx := context.Background()
	setReview(t, pool, w, true, true, true)
	setRelease(t, pool, w, "after_close", true)
	closeAssignment(t, pool, w)
	people := newCohort(t, pool, w)
	for range 3 {
		people.attempt(t, people.student(t), "graded", 5, 10)
	}
	until := later(2 * time.Hour)
	giveOverride(t, pool, w, w.student, until, nil, 0)

	store := repositories.NewPostgres(db.NewContext(pool), adapters.AttemptStartGuard{})
	record, err := store.ByID(ctx, session.Attempt.ID, w.student)
	if err != nil {
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

	held := load(time.Now())
	if held.Score != nil || held.Review.ShowScore || held.ReleasesAt == nil || !held.ReleasesAt.Equal(*until) {
		t.Errorf("past the assignment's close and before their own, the result shows %+v with release %v; want it held until %v", held.Score, held.ReleasesAt, *until)
	}
	if held.ClassAverage != nil {
		t.Errorf("a withheld result carries the average %v", *held.ClassAverage)
	}

	released := load(until.Add(time.Second))
	if released.ReleasesAt != nil || released.Score == nil || !within(released.ClassAverage, 50) {
		t.Errorf("after their own close the result is %+v (release %v, average %v), want it released with the average 50", released.Score, released.ReleasesAt, released.ClassAverage)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM app.assignment_student_overrides WHERE assignment_id = $1::uuid`, w.assignment); err != nil {
		t.Fatal(err)
	}
	if got := load(time.Now()); got.ReleasesAt != nil || got.Score == nil {
		t.Errorf("without the override the closed assignment releases the result: %+v, %v", got.Score, got.ReleasesAt)
	}
}
