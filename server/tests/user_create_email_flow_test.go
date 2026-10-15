//go:build e2e

package e2e

import (
	"context"
	"net/http"
	"testing"
)

func TestAQuotedEmailThePrinterCannotWriteBackIsRefusedByCreateUser(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	admin := w.browser()
	admin.login(email, password)
	ctx := context.Background()
	suffix := nonce(t)
	var roleID string
	if err := w.pool.QueryRow(ctx, `SELECT id::text FROM app.roles WHERE builtin_key = 'student'`).Scan(&roleID); err != nil {
		t.Fatal(err)
	}
	stored := func(address string) int {
		t.Helper()
		var count int
		if err := w.pool.QueryRow(ctx, `SELECT count(*) FROM app.users WHERE lower(email) = lower($1)`, address).Scan(&count); err != nil {
			t.Fatal(err)
		}
		return count
	}

	for _, delivery := range []string{"email_link", "temporary_password"} {
		quoted := `"qc tp ` + delivery + suffix + `"@example.com`
		unquoted := "qc tp " + delivery + suffix + "@example.com"
		status, body := admin.call(http.MethodPost, "/admin/users", map[string]any{
			"email": quoted, "fullName": "Người dùng", "roleId": roleID, "delivery": delivery,
		})
		requireEmailRefusal(t, "createUser by "+delivery+" with a quoted local part", status, body)
		if got := stored(unquoted); got != 0 {
			t.Fatalf("a refused createUser by %s left %d rows", delivery, got)
		}
	}
}
