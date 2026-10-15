//go:build e2e

package e2e

import (
	"context"
	"net/http"
	"testing"
)

func requireEmailRefusal(t *testing.T, what string, status int, body map[string]any) {
	t.Helper()
	failure, _ := body["error"].(map[string]any)
	if status != http.StatusBadRequest || failure["code"] != "VALIDATION_FAILED" {
		t.Fatalf("%s: status %d %v, want 400 VALIDATION_FAILED", what, status, body)
	}
	if want := `Trường "email" không hợp lệ.`; failure["message"] != want {
		t.Fatalf("%s: message = %v, want %q", what, failure["message"], want)
	}
}

func TestAQuotedEmailThePrinterCannotWriteBackIsRefusedAndWritesNothing(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)
	ctx := context.Background()
	suffix := nonce(t)
	stored := func(address string) int {
		t.Helper()
		var count int
		if err := w.pool.QueryRow(ctx, `SELECT count(*) FROM app.users WHERE lower(email) = lower($1)`, address).Scan(&count); err != nil {
			t.Fatal(err)
		}
		return count
	}

	quoted := `"qc tp ` + suffix + `"@example.com`
	unquoted := "qc tp " + suffix + "@example.com"
	status, body := teacher.call(http.MethodPost, "/teacher/students", map[string]any{"email": quoted, "fullName": "Học viên"})
	requireEmailRefusal(t, "createStudent with a quoted local part", status, body)
	if got := stored(unquoted); got != 0 {
		t.Fatalf("a refused createStudent left %d rows", got)
	}

	created := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/students", map[string]any{
		"email": "hoc-sinh-" + suffix + "@example.com", "fullName": "Học viên",
	})
	user := created["user"].(map[string]any)
	status, body = teacher.call(http.MethodPatch, "/teacher/students/"+id(user), map[string]any{"email": `"a..b ` + suffix + `"@example.com`})
	requireEmailRefusal(t, "updateStudent with a quoted local part", status, body)
	reloaded := teacher.must(http.StatusOK, http.MethodGet, "/teacher/students/"+id(user), nil)
	if reloaded["email"] != user["email"] {
		t.Fatalf("a refused updateStudent changed the email to %v", reloaded["email"])
	}

	anonymous := w.browser()
	status, body = anonymous.call(http.MethodPost, "/auth/login", map[string]any{"email": quoted, "password": "mat-khau-dai-du"})
	requireEmailRefusal(t, "login with a quoted local part", status, body)

	teacher.must(http.StatusOK, http.MethodGet, "/teacher/students", nil)
}

func TestAQuotedAtomIsStoredBareAndEveryListStillPrints(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)
	suffix := nonce(t)

	created := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/students", map[string]any{
		"email": `"hoc-sinh-` + suffix + `"@example.com`, "fullName": "Học viên",
	})
	if got, want := created["user"].(map[string]any)["email"], "hoc-sinh-"+suffix+"@example.com"; got != want {
		t.Fatalf("the quoted atom was stored as %v, want %q", got, want)
	}
	teacher.must(http.StatusOK, http.MethodGet, "/teacher/students", nil)
}
