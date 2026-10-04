//go:build e2e

package e2e

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/core"
	classesdomain "quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/platform/config"
)

func TestStartUpRotatesALegacyJoinCodeAndTellsItsTeacher(t *testing.T) {
	w := boot(t)
	email, password := w.createStaff("teacher")
	teacher := w.signedIn(email, password)
	name := "Lớp mã cũ " + nonce(t)
	classID := teacher.class(name)
	old, err := classesdomain.JoinCodes.Generate()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.pool.Exec(context.Background(), `
		INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, max_uses, created_by)
		VALUES ($1::uuid, $2, $3, now() + interval '30 days', 40, $4::uuid)`,
		classID, classesdomain.JoinCodes.Hash(old), classesdomain.JoinCodes.Hint(old), teacher.me(t)); err != nil {
		t.Fatalf("the v0.7.0 insert: %v", err)
	}
	path := "/teacher/classes/" + classID + "/join-code"
	if before := teacher.must(http.StatusOK, http.MethodGet, path, nil); before["legacy"] != true || before["code"] != nil {
		t.Fatalf("before start-up the code reads %v, want a legacy code nobody can read back", before)
	}

	app, err := core.New(context.Background(), config.Config{
		Port:                        "0",
		Env:                         "test",
		DatabaseURL:                 os.Getenv("TEST_DATABASE_URL"),
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
	t.Cleanup(func() {
		cancel()
		select {
		case <-served:
		case <-time.After(15 * time.Second):
			t.Error("the application did not stop")
		}
	})

	deadline := time.Now().Add(20 * time.Second)
	for {
		var legacy bool
		if err := w.pool.QueryRow(context.Background(), `
			SELECT lookup_scheme = 1 FROM app.class_join_codes
			 WHERE class_id = $1::uuid AND revoked_at IS NULL`, classID).Scan(&legacy); err != nil {
			t.Fatalf("read the class's active code: %v", err)
		}
		if !legacy {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("start-up left the legacy join code in place")
		}
		time.Sleep(20 * time.Millisecond)
	}
	current := teacher.must(http.StatusOK, http.MethodGet, path, nil)
	if current["legacy"] != false {
		t.Fatalf("after start-up the code reads %v, want one its teacher can read back", current)
	}
	fresh, _ := current["code"].(string)
	if canonical := classesdomain.JoinCodes.Normalize(fresh); len(canonical) != classesdomain.Length || canonical == old ||
		current["usesCount"] != float64(0) || current["maxUses"] != float64(40) {
		t.Fatalf("after start-up the code reads %v, want a new sealed code with the old one's cap", current)
	}

	anyone := w.browser()
	status, body := anyone.call(http.MethodPost, "/join/preview", map[string]any{"joinCode": old})
	if refusal, _ := body["error"].(map[string]any); status != http.StatusNotFound || refusal["code"] != "JOIN_CODE_REVOKED" {
		t.Errorf("the old code previews as %d %v, want 404 JOIN_CODE_REVOKED", status, body)
	}
	if preview := anyone.must(http.StatusOK, http.MethodPost, "/join/preview", map[string]any{"joinCode": fresh}); preview["classId"] != classID {
		t.Errorf("the new code previews as %v, want the class", preview)
	}

	var items []any
	for {
		items = teacher.must(http.StatusOK, http.MethodGet, "/me/notifications", nil)["items"].([]any)
		if len(items) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the teacher was never told their join code was rotated")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(items) != 1 {
		t.Fatalf("the teacher holds %d notifications, want one", len(items))
	}
	told := items[0].(map[string]any)
	if told["kind"] != "join_codes.rotated" || told["readAt"] != nil ||
		!reflect.DeepEqual(told["params"], map[string]any{"count": float64(1), "classNames": []any{name}}) ||
		!reflect.DeepEqual(told["target"], map[string]any{"route": "classes"}) {
		t.Errorf("the teacher was told %v, want join_codes.rotated for the one class, unread, opening Classes", told)
	}
	if unread := teacher.unread(t); unread != 1 {
		t.Errorf("the teacher has %v unread, want 1", unread)
	}
}
