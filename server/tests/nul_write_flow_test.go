//go:build e2e

package e2e

import (
	"context"
	"net/http"
	"testing"
)

func requireNULRefusal(t *testing.T, what string, status int, body map[string]any, field string) {
	t.Helper()
	failure, _ := body["error"].(map[string]any)
	if status != http.StatusBadRequest || failure["code"] != "VALIDATION_FAILED" {
		t.Fatalf("%s: status %d %v, want 400 VALIDATION_FAILED", what, status, body)
	}
	if details, _ := failure["details"].(map[string]any); field != "" && details[field] == nil {
		t.Fatalf("%s: details = %v, want %q named", what, failure["details"], field)
	}
}

func TestATextHoldingANulIsRefusedBeforeItCanReachPostgres(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	owner := id(teacher.login(email, password)["user"].(map[string]any))
	ctx := context.Background()

	if _, err := w.pool.Exec(ctx, `SELECT $1::text`, "a\x00b"); err == nil {
		t.Fatal("PostgreSQL accepted a NUL in a text value")
	}
	owned := func(table, column string) int {
		t.Helper()
		var count int
		if err := w.pool.QueryRow(ctx, `SELECT count(*) FROM app.`+table+` WHERE `+column+` = $1`, owner).Scan(&count); err != nil {
			t.Fatal(err)
		}
		return count
	}

	status, body := teacher.call(http.MethodPost, "/teacher/classes", map[string]any{"name": "Lớp 10\x00A"})
	requireNULRefusal(t, "a class named with a NUL", status, body, "name")
	if got := owned("classes", "teacher_id"); got != 0 {
		t.Fatalf("a refused class left %d rows", got)
	}
	teacher.must(http.StatusCreated, http.MethodPost, "/teacher/classes", map[string]any{"name": "Lớp 10A"})

	nested := map[string]any{"type": "single_choice", "prompt": "Chọn", "points": 1, "options": []any{
		map[string]any{"text": "a", "isCorrect": true}, map[string]any{"text": "b\x00", "isCorrect": false},
	}}
	status, body = teacher.call(http.MethodPost, "/teacher/questions", nested)
	requireNULRefusal(t, "a question whose second option holds a NUL", status, body, "options.1.text")
	status, body = teacher.call(http.MethodPost, "/teacher/questions", map[string]any{
		"type": "short_answer", "prompt": "Viết", "points": 1, "tags": []string{"ok", "bad\x00"},
	})
	requireNULRefusal(t, "a question whose second tag holds a NUL", status, body, "tags.1")
	if got := owned("questions", "owner_id"); got != 0 {
		t.Fatalf("refused questions left %d rows", got)
	}
	nested["options"].([]any)[1].(map[string]any)["text"] = "b"
	teacher.must(http.StatusCreated, http.MethodPost, "/teacher/questions", nested)

	for _, query := range []string{"?q=%00", "?q=ok&tag=%00"} {
		status, body = teacher.call(http.MethodGet, "/teacher/questions"+query, nil)
		requireNULRefusal(t, "a search "+query, status, body, "")
	}
	teacher.must(http.StatusOK, http.MethodGet, "/teacher/questions?q=Ch%E1%BB%8Dn", nil)

	anonymous := w.browser()
	status, body = anonymous.call(http.MethodPost, "/auth/login", map[string]any{"email": email, "password": "mat-khau\x00sai"})
	requireNULRefusal(t, "a sign-in whose password holds a NUL", status, body, "")
}
