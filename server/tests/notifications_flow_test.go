//go:build e2e

package e2e

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/core"
	"quizzivy/internal/platform/config"
)

func (c *client) me(t *testing.T) string {
	t.Helper()
	return c.must(http.StatusOK, http.MethodGet, "/auth/me", nil)["id"].(string)
}

func (c *client) unread(t *testing.T) float64 {
	t.Helper()
	return c.must(http.StatusOK, http.MethodGet, "/me/summary", nil)["unreadNotifications"].(float64)
}

func (c *client) notifications(t *testing.T, query string) (ids []string, next any) {
	t.Helper()
	page := c.must(http.StatusOK, http.MethodGet, "/me/notifications"+query, nil)
	for _, item := range page["items"].([]any) {
		ids = append(ids, id(item.(map[string]any)))
	}
	return ids, page["nextBefore"]
}

func TestASignedInUserReadsMarksAndSwitchesOnlyTheirOwnNotifications(t *testing.T) {
	w := boot(t)
	teacherEmail, teacherPassword := w.createStaff("teacher")
	studentEmail, studentPassword := w.createStaff("student")
	teacher, student := w.signedIn(teacherEmail, teacherPassword), w.signedIn(studentEmail, studentPassword)
	teacherID, studentID := teacher.me(t), student.me(t)

	for name, c := range map[string]*client{"teacher": teacher, "student": student} {
		if got := c.unread(t); got != 0 {
			t.Errorf("a new %s has %v unread", name, got)
		}
		if ids, next := c.notifications(t, ""); len(ids) != 0 || next != nil {
			t.Errorf("a new %s is shown %v (next %v)", name, ids, next)
		}
	}

	first, second := w.notification(teacherID, "A"), ""
	theirs := w.notification(studentID, "B")
	if err := w.pool.QueryRow(context.Background(), `
		INSERT INTO app.notifications (user_id, kind, params, target, dedupe_key)
		VALUES ($1::uuid, 'attempt.flagged', '{"studentName":"Lê Hoàng Nam","title":"Đề giữa kỳ","focusLost":3}'::jsonb,
		        jsonb_build_object('route', 'attempt', 'attemptId', $2::text), 'flagged')
		RETURNING id::text`, teacherID, theirs).Scan(&second); err != nil {
		t.Fatal(err)
	}

	if got := teacher.unread(t); got != 2 {
		t.Errorf("the teacher has %v unread, want 2", got)
	}
	if got := student.unread(t); got != 1 {
		t.Errorf("the student has %v unread, want 1", got)
	}
	page := teacher.must(http.StatusOK, http.MethodGet, "/me/notifications", nil)
	items := page["items"].([]any)
	if len(items) != 2 || id(items[0].(map[string]any)) != second || id(items[1].(map[string]any)) != first || page["nextBefore"] != nil {
		t.Fatalf("the teacher's list is %v, want their two rows, newest first, and no next page", page)
	}
	flagged := items[0].(map[string]any)
	if flagged["kind"] != "attempt.flagged" || flagged["readAt"] != nil ||
		flagged["params"].(map[string]any)["studentName"] != "Lê Hoàng Nam" || flagged["params"].(map[string]any)["focusLost"] != float64(3) ||
		flagged["target"].(map[string]any)["route"] != "attempt" || flagged["target"].(map[string]any)["attemptId"] != theirs {
		t.Errorf("the flagged item is %v", flagged)
	}
	if joined := items[1].(map[string]any); joined["kind"] != "class.joined" || joined["target"] != nil {
		t.Errorf("the joined item is %v, want no target", joined)
	}
	if ids, next := teacher.notifications(t, "?limit=1"); !slices.Equal(ids, []string{second}) || next != second {
		t.Errorf("a page of one is %v with next %v, want the newest row and its id", ids, next)
	}
	if ids, _ := teacher.notifications(t, "?before="+second); !slices.Equal(ids, []string{first}) {
		t.Errorf("before the newest row the teacher is shown %v, want the older one", ids)
	}
	if ids, _ := student.notifications(t, ""); !slices.Equal(ids, []string{theirs}) {
		t.Errorf("the student's list is %v, want their own row", ids)
	}
	if ids, _ := teacher.notifications(t, "?before="+theirs); slices.Contains(ids, theirs) {
		t.Errorf("a cursor at another user's row shows it: %v", ids)
	}

	teacher.must(http.StatusNoContent, http.MethodPost, "/me/notifications/read", map[string]any{"ids": []string{theirs}})
	if got := student.unread(t); got != 1 {
		t.Errorf("after the teacher marked the student's id the student has %v unread, want 1", got)
	}
	if got := teacher.unread(t); got != 2 {
		t.Errorf("marking another user's id changed the teacher's count to %v", got)
	}
	teacher.must(http.StatusNoContent, http.MethodPost, "/me/notifications/read", map[string]any{"ids": []string{first}})
	if got := teacher.unread(t); got != 1 {
		t.Errorf("the teacher has %v unread after marking one, want 1", got)
	}
	teacher.must(http.StatusNoContent, http.MethodPost, "/me/notifications/read", map[string]any{})
	if got := teacher.unread(t); got != 0 {
		t.Errorf("the teacher has %v unread after marking all, want 0", got)
	}
	if got := student.unread(t); got != 1 {
		t.Errorf("the teacher marking all left the student %v unread, want 1", got)
	}
	teacher.must(http.StatusBadRequest, http.MethodGet, "/me/notifications?limit=51", nil)

	events := []string{"attempt.submitted", "attempt.flagged", "assignment.closing", "assignment.due_soon", "result.ready"}
	switches := func(c *client) []any {
		t.Helper()
		return c.must(http.StatusOK, http.MethodGet, "/me/notification-preferences", nil)["items"].([]any)
	}
	for i, item := range switches(student) {
		if s := item.(map[string]any); s["event"] != events[i] || s["inApp"] != true || s["email"] != false {
			t.Errorf("the student's switch %d defaults to %v", i, s)
		}
	}
	saved := make([]map[string]any, len(events))
	for i, event := range events {
		saved[len(events)-1-i] = map[string]any{"event": event, "inApp": event != "result.ready", "email": event == "attempt.flagged"}
	}
	answered := student.must(http.StatusOK, http.MethodPut, "/me/notification-preferences", saved)["items"].([]any)
	stored := switches(student)
	for i, event := range events {
		for name, list := range map[string][]any{"the answer": answered, "a later read": stored} {
			if s := list[i].(map[string]any); s["event"] != event || s["inApp"] != (event != "result.ready") || s["email"] != (event == "attempt.flagged") {
				t.Errorf("%s holds %v for %s", name, s, event)
			}
		}
	}
	for i, item := range switches(teacher) {
		if s := item.(map[string]any); s["event"] != events[i] || s["inApp"] != true || s["email"] != false {
			t.Errorf("after the student saved theirs the teacher's switch %d is %v", i, s)
		}
	}
	saved[0]["event"] = "attempt.submitted"
	if status, body := student.call(http.MethodPut, "/me/notification-preferences", saved); status != http.StatusBadRequest {
		t.Errorf("a repeated switch answered %d %v, want 400", status, body)
	}
}

func TestTheApplicationStartsItsJobsAndStops(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; the end-to-end suite needs a database")
	}
	app, err := core.New(context.Background(), config.Config{
		Port:                        "0",
		Env:                         "test",
		DatabaseURL:                 dsn,
		AllowedOrigins:              []string{"http://localhost:5173"},
		MaxConcurrentPasswordHashes: 4,
		JWTSigningKey:               []byte(strings.Repeat("e2e-signing-key-", 2)),
		JoinCodeKey:                 []byte(strings.Repeat("e2e-join-code-k", 2) + "ey"),
		AccessTokenTTL:              15 * time.Minute,
		RefreshTokenTTL:             30 * 24 * time.Hour,
	}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("assemble the application: %v", err)
	}
	t.Cleanup(app.Close)
	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- app.Serve(ctx) }()
	select {
	case err := <-served:
		t.Fatalf("the application stopped by itself: %v", err)
	case <-time.After(500 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-served:
		if err != nil {
			t.Errorf("shutting down: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("the application did not stop")
	}
}
