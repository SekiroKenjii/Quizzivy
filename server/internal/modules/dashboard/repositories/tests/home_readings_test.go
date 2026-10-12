//go:build integration

package repositories_test

import (
	"context"
	"quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/modules/dashboard/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"reflect"
	"testing"
	"time"
)

func homeAt(t *testing.T, w *homeWorld, scope access.Scope, now time.Time, zone string, days int) domain.Home {
	t.Helper()
	out, err := repositories.NewPostgres(db.NewContext(w.tx)).Home(context.Background(), domain.HomeQuery{Scope: scope, Now: now, Zone: zone, Days: days})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func fixedNow() time.Time { return time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC) }
func execHome(t *testing.T, w *homeWorld, sql string, args ...any) {
	t.Helper()
	if _, err := w.tx.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func TestCalendarRangesPadDaysAndAverageOnlyScoredGradedPapers(t *testing.T) {
	w := newHomeWorld(t)
	now := fixedNow()
	scope := access.Scope{UserID: w.a}
	execHome(t, w, `UPDATE app.attempts SET submitted_at=$2,score_earned=1,score_total=1 WHERE id=$1`, w.p2, now.AddDate(0, 0, -20))
	execHome(t, w, `UPDATE app.attempts SET status='graded',graded_at=$2,submitted_at=$3,score_earned=1,score_total=2 WHERE id=$1`, w.p3, now, now.AddDate(0, 0, -6))
	execHome(t, w, `UPDATE app.attempts SET status='graded',graded_at=$2,submitted_at=$2,score_earned=3,score_total=4 WHERE id=$1`, w.p4, now)
	execHome(t, w, `UPDATE app.attempts SET assignment_id=$2,status='graded',graded_at=$3,submitted_at=$3,score_earned=NULL,score_total=NULL WHERE id=$1`, w.p6, w.aA, now)
	execHome(t, w, `UPDATE app.attempts SET assignment_id=$2,attempt_no=2,status='submitted',submitted_at=$3,score_earned=1,score_total=1 WHERE id=$1`, w.p10, w.aA, now)
	execHome(t, w, `UPDATE app.attempts SET assignment_id=$2,status='voided',void_reason='test',submitted_at=$3 WHERE id=$1`, w.p5, w.aA, now)
	execHome(t, w, `UPDATE app.attempts SET assignment_id=$2,status='in_progress',submitted_at=NULL,started_at=$3,deadline_at=$4 WHERE id=$1`, w.p7, w.aA, now.Add(-time.Hour), now.Add(time.Hour))
	for _, days := range []int{7, 14, 30} {
		out := homeAt(t, w, scope, now, "Asia/Ho_Chi_Minh", days).Submissions
		want := 4
		if days == 30 {
			want = 5
		}
		if len(out.Days) != days || out.Total != want || out.AveragePercent == nil || *out.AveragePercent != 63 {
			t.Fatalf("%dd result=%+v", days, out)
		}
		sum := 0
		for i, day := range out.Days {
			wantDate := now.AddDate(0, 0, -days+1+i).Format("2006-01-02")
			if day.Date != wantDate {
				t.Fatalf("day %d=%s want%s", i, day.Date, wantDate)
			}
			sum += day.Count
		}
		if sum != out.Total || out.Days[len(out.Days)-1].Count != 3 || out.Days[1].Count != 0 {
			t.Fatalf("series=%+v", out)
		}
	}
	execHome(t, w, `UPDATE app.attempts SET score_earned=NULL,score_total=NULL WHERE id=ANY($1::uuid[])`, []string{w.p3, w.p4})
	if out := homeAt(t, w, scope, now, "Asia/Ho_Chi_Minh", 14).Submissions; out.AveragePercent != nil || out.Total != 4 {
		t.Fatalf("nullable graded=%+v", out)
	}
}

func TestCalendarMidnightAndTodayUseTheActorsZone(t *testing.T) {
	for _, test := range []struct {
		zone     string
		boundary time.Time
	}{{"Asia/Ho_Chi_Minh", time.Date(2026, 10, 4, 17, 0, 0, 0, time.UTC)}, {"Asia/Tokyo", time.Date(2026, 10, 4, 15, 0, 0, 0, time.UTC)}, {"America/New_York", time.Date(2026, 11, 1, 4, 0, 0, 0, time.UTC)}} {
		t.Run(test.zone, func(t *testing.T) {
			w := newHomeWorld(t)
			scope := access.Scope{UserID: w.a}
			before := test.boundary.Add(-time.Second)
			execHome(t, w, `UPDATE app.attempts SET submitted_at=$2 WHERE id=ANY($1::uuid[])`, []string{w.p2, w.p3}, before)
			execHome(t, w, `UPDATE app.attempts SET submitted_at=$2 WHERE id=$1`, w.p4, test.boundary)
			execHome(t, w, `UPDATE app.assignments SET opens_at=$2,closes_at=$3,closed_at=NULL WHERE id=$1`, w.aA, test.boundary, test.boundary.Add(48*time.Hour))
			old := homeAt(t, w, scope, before, test.zone, 7)
			fresh := homeAt(t, w, scope, test.boundary, test.zone, 7)
			if old.Submissions.Days[6].Date != "2026-10-04" && test.zone != "America/New_York" {
				t.Fatalf("old date=%s", old.Submissions.Days[6].Date)
			}
			if old.Submissions.Days[6].Count != 2 || fresh.Submissions.Days[5].Count != 2 || fresh.Submissions.Days[6].Count != 1 {
				t.Fatalf("boundary old=%+v fresh=%+v", old.Submissions, fresh.Submissions)
			}
			seenOld, seenNew := false, false
			for _, event := range old.Today {
				seenOld = seenOld || event.AssignmentID == w.aA
			}
			for _, event := range fresh.Today {
				seenNew = seenNew || event.AssignmentID == w.aA && event.Kind == "opens"
			}
			if seenOld || !seenNew {
				t.Fatalf("today old=%+v fresh=%+v", old.Today, fresh.Today)
			}
			if test.zone == "America/New_York" {
				execHome(t, w, `UPDATE app.attempts SET submitted_at=$2 WHERE id=$1`, w.p4, test.boundary.Add(24*time.Hour+30*time.Minute))
				out := homeAt(t, w, scope, test.boundary.Add(24*time.Hour), test.zone, 7)
				if out.Submissions.Days[6].Date != "2026-11-01" || out.Submissions.Days[6].Count != 1 {
					t.Fatalf("25-hour day=%+v", out.Submissions)
				}
			}
		})
	}
}

func TestTakingNowExcludesTheDeadlineAndCountsDistinctSubjects(t *testing.T) {
	w := newHomeWorld(t)
	now := fixedNow()
	scope := access.Scope{UserID: w.b}
	for _, id := range []string{w.p1, w.p6, w.p7} {
		execHome(t, w, `UPDATE app.attempts SET status='in_progress',submitted_at=NULL,started_at=$2,deadline_at=$3 WHERE id=$1`, id, now.Add(-time.Hour), now.Add(time.Hour))
	}
	execHome(t, w, `UPDATE app.attempts SET student_id=$2 WHERE id=$1`, w.p6, w.s1)
	execHome(t, w, `UPDATE app.attempts SET deadline_at=$2 WHERE id=$1`, w.p7, now)
	out := homeAt(t, w, scope, now, "Asia/Ho_Chi_Minh", 14)
	expectedAssignment := w.aB
	if w.p6 > w.p1 {
		expectedAssignment = w.aB2
	}
	if !reflect.DeepEqual(out.TakingNow, domain.TakingNow{Students: 1, Assignments: 2, AssignmentID: &expectedAssignment}) {
		t.Fatalf("taking=%+v", out.TakingNow)
	}
	execHome(t, w, `UPDATE app.attempts SET deadline_at=$2 WHERE id=$1`, w.p7, now.Add(-time.Second))
	if got := homeAt(t, w, scope, now, "Asia/Ho_Chi_Minh", 14).TakingNow; !reflect.DeepEqual(got, out.TakingNow) {
		t.Fatalf("past deadline=%+v", got)
	}
}

func TestHomeRangeChangesOnlySubmissionsAndKeepsOwnAndWideReadings(t *testing.T) {
	w := newHomeWorld(t)
	now := fixedNow()
	own := access.Scope{UserID: w.admin}
	wide := access.Scope{UserID: w.admin, All: true}
	a := homeAt(t, w, own, now, "Asia/Ho_Chi_Minh", 7)
	b := homeAt(t, w, own, now, "Asia/Ho_Chi_Minh", 30)
	a.Submissions = domain.Submissions{}
	b.Submissions = domain.Submissions{}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("range changed other readings: %+v %+v", a, b)
	}
	before := homeAt(t, w, access.Scope{UserID: w.a}, now, "Asia/Ho_Chi_Minh", 14)
	execHome(t, w, `UPDATE app.attempts SET flagged=NOT flagged,submitted_at=$2 WHERE id=$1`, w.p1, now)
	after := homeAt(t, w, access.Scope{UserID: w.a}, now, "Asia/Ho_Chi_Minh", 14)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("B changed A home: before%+v after%+v", before, after)
	}
	gotOwn := homeAt(t, w, own, now, "Asia/Ho_Chi_Minh", 14)
	gotWide := homeAt(t, w, wide, now, "Asia/Ho_Chi_Minh", 14)
	if gotWide.Submissions.Total <= gotOwn.Submissions.Total {
		t.Fatalf("own=%+v wide=%+v", gotOwn.Submissions, gotWide.Submissions)
	}
	if zero := homeAt(t, w, access.Scope{}, now, "Asia/Ho_Chi_Minh", 14); zero.Submissions.Total != 0 || len(zero.Today) != 0 || len(zero.RecentActivity) != 0 || zero.TakingNow != (domain.TakingNow{}) {
		t.Fatalf("zero scope=%+v", zero)
	}
}
