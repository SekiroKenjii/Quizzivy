//go:build integration

package repositories_test

import (
	"context"
	"slices"
	"testing"

	"quizzivy/internal/modules/notifications/application"
	"quizzivy/internal/modules/notifications/application/command"
	"quizzivy/internal/modules/notifications/application/query"
	"quizzivy/internal/modules/notifications/domain"
)

func unreadOf(t *testing.T, app *application.Application, userID string, audience domain.Audience) int {
	t.Helper()
	summary, err := app.Queries.Summary.Handle(context.Background(), query.Summary{UserID: userID, Audience: audience})
	if err != nil {
		t.Fatalf("summary for %q: %v", audience, err)
	}
	return summary.UnreadNotifications
}

func pageOf(t *testing.T, app *application.Application, userID, before string, limit int, audience domain.Audience) domain.Page {
	t.Helper()
	got, err := app.Queries.List.Handle(context.Background(), query.List{UserID: userID, Before: before, Limit: limit, Audience: audience})
	if err != nil {
		t.Fatalf("list for %q: %v", audience, err)
	}
	return got
}

func markOf(t *testing.T, app *application.Application, userID string, audience domain.Audience, ids ...string) {
	t.Helper()
	var err error
	if ids == nil {
		_, err = app.Commands.MarkAllRead.Handle(context.Background(), command.MarkAllRead{UserID: userID, Audience: audience})
	} else {
		_, err = app.Commands.MarkRead.Handle(context.Background(), command.MarkRead{UserID: userID, IDs: ids, Audience: audience})
	}
	if err != nil {
		t.Fatalf("mark for %q: %v", audience, err)
	}
}

func holdsBothConsoles(t *testing.T, app *application.Application, userID string, teacher, student int) {
	t.Helper()
	for i := 0; teacher+student > 0; i++ {
		if student > 0 && (i%5 == 4 || teacher == 0) {
			student--
			notify(t, app, userID, "ready:"+nonce(t), domain.Ready{Title: "Đề giữa kỳ"}, domain.Replace)
			continue
		}
		teacher--
		notify(t, app, userID, "joined:"+nonce(t), domain.Joined{StudentName: "Nguyễn Gia Bảo", ClassName: "Lớp A"}, domain.Replace)
	}
}

func kindsOf(rows []stored) map[string]int {
	out := map[string]int{}
	for _, r := range rows {
		out[r.kind]++
	}
	return out
}

func TestEachBellCountsListsAndClearsOnlyItsOwnKinds(t *testing.T) {
	pool := newPool(t)
	app := over(pool)
	userID := newUser(t, pool, "teacher")
	holdsBothConsoles(t, app, userID, 20, 5)
	if got := kindsOf(rowsOf(t, pool, userID)); got["class.joined"] != 20 || got["result.ready"] != 5 {
		t.Fatalf("the fixture holds %v, want 20 teacher and 5 student rows", got)
	}

	for audience, want := range map[domain.Audience]int{"": 25, domain.AudienceTeacher: 20, domain.AudienceStudent: 5} {
		if got := unreadOf(t, app, userID, audience); got != want {
			t.Errorf("%q counts %d unread, want %d", audience, got, want)
		}
	}

	var seen []string
	before := ""
	for pages := 0; ; pages++ {
		got := pageOf(t, app, userID, before, 8, domain.AudienceTeacher)
		for _, n := range got.Items {
			if n.Kind.Audience() != domain.AudienceTeacher {
				t.Errorf("the teacher's list holds %s", n.Kind)
			}
			seen = append(seen, n.ID)
		}
		if got.NextBefore == "" {
			if pages != 2 {
				t.Errorf("the teacher's 20 rows took %d pages of 8, want 3", pages+1)
			}
			break
		}
		if len(got.Items) != 8 {
			t.Errorf("a page that has a next one holds %d rows, want 8 of the teacher's own", len(got.Items))
		}
		before = got.NextBefore
	}
	if len(seen) != 20 || len(slices.Compact(slices.Sorted(slices.Values(seen)))) != 20 {
		t.Errorf("the teacher's pages held %d rows, want its 20 once each", len(seen))
	}
	if students := pageOf(t, app, userID, "", 8, domain.AudienceStudent); len(students.Items) != 5 || students.NextBefore != "" {
		t.Errorf("the student's list holds %d rows (next %q), want its 5 on one page", len(students.Items), students.NextBefore)
	}
	if every := pageOf(t, app, userID, "", 50, ""); len(every.Items) != 25 {
		t.Errorf("a list with no audience holds %d rows, want all 25", len(every.Items))
	}

	markOf(t, app, userID, domain.AudienceStudent)
	if got := unreadOf(t, app, userID, domain.AudienceStudent); got != 0 {
		t.Errorf("the student's bell has %d unread after clearing, want 0", got)
	}
	if got := unreadOf(t, app, userID, domain.AudienceTeacher); got != 20 {
		t.Errorf("clearing the student's bell left the teacher's with %d unread, want 20", got)
	}
	for _, r := range rowsOf(t, pool, userID) {
		if (r.kind == "result.ready") != (r.readAt != nil) {
			t.Errorf("%s is read: %v after the student's bell cleared", r.kind, r.readAt != nil)
		}
	}
}

func TestMarkingByIdSkipsTheOtherConsolesRows(t *testing.T) {
	pool := newPool(t)
	app := over(pool)
	userID := newUser(t, pool, "teacher")
	holdsBothConsoles(t, app, userID, 4, 1)
	var teacherIDs, studentIDs []string
	for _, r := range rowsOf(t, pool, userID) {
		if r.kind == "result.ready" {
			studentIDs = append(studentIDs, r.id)
		} else {
			teacherIDs = append(teacherIDs, r.id)
		}
	}

	markOf(t, app, userID, domain.AudienceTeacher, studentIDs...)
	if got := unreadOf(t, app, userID, domain.AudienceStudent); got != 1 {
		t.Errorf("the teacher's bell marked the student's row by id: %d unread, want 1", got)
	}
	markOf(t, app, userID, domain.AudienceStudent, slices.Concat(studentIDs, teacherIDs[:2])...)
	if got, want := unreadOf(t, app, userID, domain.AudienceStudent), 0; got != want {
		t.Errorf("the student's bell left %d of its own unread, want %d", got, want)
	}
	if got := unreadOf(t, app, userID, domain.AudienceTeacher); got != 4 {
		t.Errorf("the student's bell marked teacher rows by id: %d unread, want 4", got)
	}
	markOf(t, app, userID, "", teacherIDs[:2]...)
	if got := unreadOf(t, app, userID, ""); got != 2 {
		t.Errorf("marking two by id with no audience leaves %d unread, want 2", got)
	}
}

func TestAnAudienceNeverReachesAnotherUsersRows(t *testing.T) {
	pool := newPool(t)
	app := over(pool)
	a, b := newUser(t, pool, "teacher"), newUser(t, pool, "teacher")
	holdsBothConsoles(t, app, a, 3, 2)
	holdsBothConsoles(t, app, b, 3, 2)
	theirs := idsOf(rowsOf(t, pool, b))

	markOf(t, app, a, domain.AudienceTeacher, theirs...)
	markOf(t, app, a, domain.AudienceStudent)
	markOf(t, app, a, domain.AudienceTeacher)
	for audience, want := range map[domain.Audience]int{"": 5, domain.AudienceTeacher: 3, domain.AudienceStudent: 2} {
		if got := unreadOf(t, app, b, audience); got != want {
			t.Errorf("after the other user cleared both bells and marked its ids, %q has %d unread, want %d", audience, got, want)
		}
		if got := unreadOf(t, app, a, audience); got != 0 {
			t.Errorf("the user who cleared both bells has %q %d unread, want 0", audience, got)
		}
	}
	for _, n := range pageOf(t, app, a, "", 50, domain.AudienceStudent).Items {
		if slices.Contains(theirs, n.ID) {
			t.Errorf("the list of one user holds %s of another", n.ID)
		}
	}
}
