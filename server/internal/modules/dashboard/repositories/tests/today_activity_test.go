//go:build integration

package repositories_test

import (
	"context"
	"quizzivy/internal/modules/assignments/domain"
	assignmentrepo "quizzivy/internal/modules/assignments/repositories"
	dashboarddomain "quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/modules/dashboard/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"testing"
	"time"
)

func TestTodayIsAscendingUsesEarlyCloseAndCountsReachedMissingTargets(t *testing.T) {
	w := newHomeWorld(t)
	now := fixedNow()
	scope := access.Scope{UserID: w.b}
	start := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	execHome(t, w, `UPDATE app.assignments SET opens_at=$2,closes_at=$3,closed_at=$4 WHERE id=$1`, w.aB, start, start.Add(36*time.Hour), start.Add(2*time.Hour))
	execHome(t, w, `UPDATE app.assignments SET opens_at=$2,closes_at=$3 WHERE id=$1`, w.aB2, start.Add(time.Hour), start.Add(3*time.Hour))
	execHome(t, w, `INSERT INTO app.assignment_students (assignment_id,user_id) VALUES ($1,$2),($1,$3)`, w.aB, w.s1, w.s4)
	execHome(t, w, `INSERT INTO app.class_members (class_id,user_id,joined_via,added_by) VALUES ($1,$2,'admin',$3)`, w.classB, w.s4, w.b)
	execHome(t, w, `DELETE FROM app.class_members WHERE class_id=$1 AND user_id=$2`, w.classB, w.s8)
	execHome(t, w, `UPDATE app.attempts SET assignment_id=$2 WHERE id=$1`, w.p7, w.aB)
	execHome(t, w, `UPDATE app.attempts SET status='voided',void_reason='test' WHERE id=$1`, w.p6)
	out := homeAt(t, w, scope, now, "Asia/Ho_Chi_Minh", 14)
	found := map[string]int{}
	last := time.Time{}
	for _, event := range out.Today {
		if event.At.Before(last) {
			t.Fatalf("today is descending: %+v", out.Today)
		}
		last = event.At
		if event.AssignmentID == w.aB {
			found[event.Kind]++
			if event.Kind == "closes" && !event.At.Equal(start.Add(2*time.Hour)) {
				t.Fatalf("effective close=%+v", event)
			}
			if event.NotSubmitted != 1 {
				t.Fatalf("disabled, handed-in or historical outside target counted: %+v", event)
			}
		}
		if event.AssignmentID == w.aB2 && event.NotSubmitted != 1 {
			t.Fatalf("voided target should remain missing: %+v", event)
		}
	}
	if found["opens"] != 1 || found["closes"] != 1 {
		t.Fatalf("early close events=%+v", out.Today)
	}
	for i := 0; i < 25; i++ {
		id := w.assignment(t, w.b, "-2 hours", "2 hours", nil, nil)
		execHome(t, w, `UPDATE app.assignments SET opens_at=$2,closes_at=$3 WHERE id=$1`, id, start.Add(time.Duration(i)*time.Minute), start.Add(4*time.Hour))
	}
	capped := homeAt(t, w, scope, now, "Asia/Ho_Chi_Minh", 14)
	if len(capped.Today) != 20 {
		t.Fatalf("today cap=%d", len(capped.Today))
	}
}

func TestRecentActivityHasOneRowPerPaperAndCodeJoinsOnly(t *testing.T) {
	w := newHomeWorld(t)
	now := fixedNow()
	scope := access.Scope{UserID: w.a}
	for i, id := range []string{w.p2, w.p3, w.p4} {
		execHome(t, w, `UPDATE app.attempts SET submitted_at=$2 WHERE id=$1`, id, now.Add(-time.Duration(i)*time.Minute))
	}
	execHome(t, w, `UPDATE app.users SET full_name=$2 WHERE id=$1`, w.s2, "submitted-student")
	execHome(t, w, `UPDATE app.attempts SET status='in_progress',submitted_at=NULL,started_at=$2,deadline_at=$3 WHERE id=$1`, w.p3, now.Add(-time.Hour), now.Add(time.Hour))
	execHome(t, w, `UPDATE app.users SET full_name=$2 WHERE id=$1`, w.s3, "code-student")
	execHome(t, w, `UPDATE app.classes SET name=$2 WHERE id=$1`, w.classA, "Code class")
	code := w.id(t, `INSERT INTO app.class_join_codes (class_id,code_hash,code_hint,expires_at,max_uses,created_by) VALUES ($1,sha256($2::bytea),'test',now()+interval '1 day',100,$3) RETURNING id::text`, w.classA, nonce(t), w.a)
	execHome(t, w, `UPDATE app.class_members SET joined_via='join_code',join_code_id=$3,added_by=NULL,joined_at=$4 WHERE class_id=$1 AND user_id=$2`, w.classA, w.s3, code, now.Add(time.Minute))
	out := homeAt(t, w, scope, now, "Asia/Ho_Chi_Minh", 14)
	submitted, started, joined := 0, 0, 0
	for i, event := range out.RecentActivity {
		if i > 0 && event.At.After(out.RecentActivity[i-1].At) {
			t.Fatal("activity not newest first")
		}
		if event.Kind == "submitted" {
			submitted++
			if !event.Flagged {
				t.Fatalf("flag lost: %+v", event)
			}
		}
		if event.Kind == "started" {
			started++
		}
		if event.Kind == "joined" {
			joined++
			if event.Flagged || event.StudentName != "code-student" || event.Subject != "Code class" || !event.At.Equal(now.Add(time.Minute)) {
				t.Fatalf("wrong code join: %+v", event)
			}
		}
	}
	if submitted != 2 || started != 1 || joined != 1 || out.RecentActivity[0].Kind != "joined" {
		t.Fatalf("activity=%+v", out.RecentActivity)
	}
	for i := 0; i < 13; i++ {
		student := w.user(t, "student", nil)
		assignment := w.assignment(t, w.a, "-2 hours", "2 hours", nil, nil)
		id := w.paper(t, assignment, student, "1 hour", true, false)
		execHome(t, w, `UPDATE app.attempts SET submitted_at=$2 WHERE id=$1`, id, now.Add(time.Duration(i+2)*time.Minute))
	}
	if got := homeAt(t, w, scope, now, "Asia/Ho_Chi_Minh", 14).RecentActivity; len(got) != 10 {
		t.Fatalf("activity cap=%d", len(got))
	}
}

func TestNavCountsMatchTheOpenListAndKeepTeacherAndAdminReach(t *testing.T) {
	w := newHomeWorld(t)
	store := repositories.NewPostgres(db.NewContext(w.tx))
	var now time.Time
	if err := w.tx.QueryRow(context.Background(), `SELECT now()`).Scan(&now); err != nil {
		t.Fatal(err)
	}
	for _, scope := range []access.Scope{{UserID: w.a}, {UserID: w.b}, {UserID: w.admin}, {UserID: w.admin, All: true}} {
		got, err := store.LiveAssignments(context.Background(), scope, now)
		if err != nil {
			t.Fatal(err)
		}
		status := domain.Open
		_, page, err := assignmentrepo.NewPostgres(db.NewContext(w.tx)).List(context.Background(), domain.ListInput{Scope: scope, Status: &status})
		if err != nil {
			t.Fatal(err)
		}
		if got != page.Total {
			t.Fatalf("scope%+v nav%d list%d", scope, got, page.Total)
		}
		summary, err := store.Summary(context.Background(), dashboarddomain.SummaryQuery{Scope: scope})
		if err != nil {
			t.Fatal(err)
		}
		grade, err := store.AnswersToGrade(context.Background(), scope)
		if err != nil {
			t.Fatal(err)
		}
		if grade != summary.AwaitingGrading {
			t.Fatalf("grading%d legacy%d", grade, summary.AwaitingGrading)
		}
	}
	before, err := store.AnswersToGrade(context.Background(), access.Scope{UserID: w.a})
	if err != nil {
		t.Fatal(err)
	}
	execHome(t, w, `UPDATE app.attempt_answers SET manual_score=1,graded_at=now() WHERE attempt_id=$1`, w.p1)
	after, err := store.AnswersToGrade(context.Background(), access.Scope{UserID: w.a})
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("B grading changed A count %d->%d", before, after)
	}
}
