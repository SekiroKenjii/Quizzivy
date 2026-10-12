//go:build integration

package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/core/adapters"
	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/modules/attempts/repositories"
	notificationsdomain "quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/platform/db"
)

func assignmentOf(t *testing.T, pool *pgxpool.Pool, attempt string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `SELECT assignment_id::text FROM app.attempts WHERE id = $1::uuid`, attempt).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func gradeAndFinish(t *testing.T, svc *application.Application, p paper) error {
	t.Helper()
	ctx := context.Background()
	if _, err := svc.Commands.Grade.Handle(ctx, command.Grade{
		AttemptID: p.attempt, GraderID: p.admin, Scope: everyone,
		Items: []domain.GradeItem{{QuestionID: p.essay, Points: 2.5}},
	}); err != nil {
		t.Fatalf("grade: %v", err)
	}
	_, err := svc.Commands.Finish.Handle(ctx, command.Finish{AttemptID: p.attempt, Scope: everyone})
	return err
}

func TestFinishingAPaperTellsTheStudentWhenTheReviewPolicyShowsTheScoreNow(t *testing.T) {
	cases := []struct {
		name   string
		policy string
		want   bool
	}{
		{"released on submit, score shown", `review_release = 'on_submit'`, true},
		{"released on submit, score hidden", `review_release = 'on_submit', review_show_score = false`, false},
		{"released after a close still ahead", `review_release = 'after_close'`, false},
		{"released after a close that has passed", `review_release = 'after_close', closed_at = now() - interval '1 minute'`, true},
		{"after a passed close but score hidden", `review_release = 'after_close', review_show_score = false, closed_at = now() - interval '1 minute'`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pool := newPool(t)
			p := seedPaper(t, pool, "submitted")
			if _, err := pool.Exec(t.Context(), `UPDATE app.assignments SET `+c.policy+` WHERE id = $1::uuid`, assignmentOf(t, pool, p.attempt)); err != nil {
				t.Fatal(err)
			}
			tell := &told{}
			svc := announcing(pool, tell, nil)
			if err := gradeAndFinish(t, svc, p); err != nil {
				t.Fatalf("finish: %v", err)
			}
			sent := tell.of(notificationsdomain.ResultReady)
			if (len(sent) == 1) != c.want || tell.count() != len(sent) {
				t.Fatalf("sent %+v, want a result notice = %v and nothing else", tell.sent, c.want)
			}
			if !c.want {
				return
			}
			n := sent[0]
			if n.UserID != p.student || n.Params != (notificationsdomain.Ready{Title: "Unit 5"}) {
				t.Errorf("notice %+v, want the student told the title and nothing more", n)
			}
			if want := (notificationsdomain.Target{Route: notificationsdomain.RouteResult, AttemptID: p.attempt, AssignmentID: assignmentOf(t, pool, p.attempt)}); *n.Target != want {
				t.Errorf("target %+v, want %+v", *n.Target, want)
			}
			if n.DedupeKey != "ready:"+p.attempt || n.Merge != notificationsdomain.Replace {
				t.Errorf("key %q, merge %d", n.DedupeKey, n.Merge)
			}
		})
	}
}

func TestAStudentWhoseOverrideKeepsTheTestOpenIsNotToldTheResultIsReady(t *testing.T) {
	pool := newPool(t)
	p := seedPaper(t, pool, "submitted")
	assignment := assignmentOf(t, pool, p.attempt)
	if _, err := pool.Exec(t.Context(), `UPDATE app.assignments SET review_release = 'after_close', closed_at = now() - interval '1 minute' WHERE id = $1::uuid`, assignment); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), `INSERT INTO app.assignment_student_overrides (assignment_id, student_id, closes_at, reason)
		VALUES ($1::uuid, $2::uuid, now() + interval '1 day', 'Ốm')`, assignment, p.student); err != nil {
		t.Fatal(err)
	}
	tell := &told{}
	if err := gradeAndFinish(t, announcing(pool, tell, nil), p); err != nil {
		t.Fatal(err)
	}
	if tell.count() != 0 {
		t.Errorf("a student who may still sit the test was told %+v", tell.sent)
	}
}

func TestARefusedFinishTellsNobodyAndAnotherFinishTellsAgain(t *testing.T) {
	pool := newPool(t)
	p := seedPaper(t, pool, "submitted")
	tell := &told{}
	svc := announcing(pool, tell, nil)
	if _, err := svc.Commands.Finish.Handle(t.Context(), command.Finish{AttemptID: p.attempt, Scope: everyone}); !errors.Is(err, domain.ErrGradingIncomplete) {
		t.Fatalf("finish with an essay unread: %v", err)
	}
	if tell.count() != 0 {
		t.Fatalf("a refused finish told %+v", tell.sent)
	}
	if err := gradeAndFinish(t, svc, p); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Commands.Finish.Handle(t.Context(), command.Finish{AttemptID: p.attempt, Scope: everyone}); err != nil {
		t.Fatal(err)
	}
	sent := tell.of(notificationsdomain.ResultReady)
	if len(sent) != 2 || sent[0].DedupeKey != sent[1].DedupeKey {
		t.Errorf("two finishes sent %+v, want two notices under one key, so the store holds one row", sent)
	}
}

func TestTheResultNoticeThatReachesTheStoreHoldsNoScore(t *testing.T) {
	pool := newPool(t)
	p := seedPaper(t, pool, "submitted")
	notifications := realNotifications(pool)
	svc := announcing(pool, notifications.Commands.Notify, nil)
	if err := gradeAndFinish(t, svc, p); err != nil {
		t.Fatal(err)
	}
	var params string
	if err := pool.QueryRow(t.Context(), `SELECT params::text FROM app.notifications WHERE user_id = $1::uuid AND kind = 'result.ready'`, p.student).Scan(&params); err != nil {
		t.Fatalf("the student holds no result notice: %v", err)
	}
	if params != `{"title": "Unit 5"}` {
		t.Errorf("the stored params are %s, want the title alone", params)
	}
}

func TestTheBriefingShowsTheScoreFromTheInstantOfTheStudentsOwnClose(t *testing.T) {
	pool := newPool(t)
	p := seedPaper(t, pool, "submitted")
	assignment := assignmentOf(t, pool, p.attempt)
	closes := time.Date(2026, 10, 10, 9, 30, 0, 0, time.UTC)
	if _, err := pool.Exec(t.Context(), `UPDATE app.assignments SET review_release = 'after_close', opens_at = $2::timestamptz - interval '3 hours', closes_at = $2::timestamptz + interval '1 day', closed_at = $2::timestamptz WHERE id = $1::uuid`, assignment, closes); err != nil {
		t.Fatal(err)
	}
	store := repositories.NewPostgres(db.NewContext(pool), adapters.AttemptStartGuard{})
	for name, c := range map[string]struct {
		at   time.Time
		want bool
	}{
		"a microsecond before the close": {closes.Add(-time.Microsecond), false},
		"the instant of the close":       {closes, true},
		"a minute after":                 {closes.Add(time.Minute), true},
	} {
		brief, err := store.Briefing(t.Context(), p.attempt, c.at)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if brief.ShowsResult != c.want {
			t.Errorf("%s: the score is shown = %v, want %v", name, brief.ShowsResult, c.want)
		}
	}
}

func TestTheBriefingOfAVoidedOrMissingAttemptIsNotFound(t *testing.T) {
	pool := newPool(t)
	p := seedPaper(t, pool, "voided")
	store := repositories.NewPostgres(db.NewContext(pool), adapters.AttemptStartGuard{})
	if _, err := store.Briefing(t.Context(), p.attempt, time.Now()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("a voided attempt: %v, want ErrNotFound", err)
	}
	if _, err := store.Briefing(t.Context(), "01935000-0000-7000-8000-00000000dead", time.Now()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("a missing attempt: %v, want ErrNotFound", err)
	}
}
