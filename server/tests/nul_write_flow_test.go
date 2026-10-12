//go:build e2e

package e2e

import (
	"bytes"
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

func nulFilenameUpload(t *testing.T, encodedName string) *payload {
	t.Helper()
	var b bytes.Buffer
	b.WriteString("--b\r\nContent-Disposition: form-data; name=\"file\"; filename*=UTF-8''" + encodedName + "\r\nContent-Type: image/png\r\n\r\n")
	b.Write(tinyPNG(t))
	b.WriteString("\r\n--b--\r\n")
	return &payload{contentType: "multipart/form-data; boundary=b", data: b.Bytes()}
}

func TestAFileNameHoldingANulIsRefusedBeforeItCanReachPostgres(t *testing.T) {
	w := bootWithStorage(t)
	email, password := w.createStaff("teacher")
	teacher := w.signedIn(email, password)
	ctx := context.Background()
	var assets int
	count := func() int {
		t.Helper()
		if err := w.pool.QueryRow(ctx, `SELECT count(*) FROM app.media_assets WHERE uploaded_by = (SELECT id FROM app.users WHERE email = $1)`, email).Scan(&assets); err != nil {
			t.Fatal(err)
		}
		return assets
	}

	refused := teacher.send(http.MethodPost, "/teacher/media", nulFilenameUpload(t, "a%00b.png"))
	requireNULRefusal(t, "an upload named with a NUL (RFC 2231)", refused.status, refused.json, "filename")
	if got := count(); got != 0 {
		t.Fatalf("a refused upload left %d rows", got)
	}

	stored := teacher.send(http.MethodPost, "/teacher/media", nulFilenameUpload(t, "b%C3%A0i.png"))
	if stored.status != http.StatusCreated || stored.json["originalFilename"] != "bài.png" {
		t.Fatalf("the same upload without the NUL: %s", answer(stored))
	}
	replaced := teacher.send(http.MethodPost, "/teacher/media/"+id(stored.json)+"/replace", nulFilenameUpload(t, "c%00d.png"))
	requireNULRefusal(t, "a replacement named with a NUL", replaced.status, replaced.json, "filename")
	if got := count(); got != 1 {
		t.Fatalf("a refused replacement changed the library to %d rows, want the one upload", got)
	}
}
