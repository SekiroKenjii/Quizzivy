//go:build e2e

package e2e

import (
	"context"
	"net/http"
	"slices"
	"testing"
)

func TestEachBuiltInStaffRoleSignsInWithItsEpochAndWorkspaces(t *testing.T) {
	w := boot(t)
	for _, c := range []struct {
		builtin     string
		workspaces  []string
		deleteUser  int
		manageUsers bool
	}{
		{builtin: "admin", workspaces: []string{"teacher", "admin"}, deleteUser: http.StatusNotFound, manageUsers: true},
		{builtin: "teacher", workspaces: []string{"teacher"}, deleteUser: http.StatusForbidden},
		{builtin: "assistant", workspaces: []string{"teacher"}, deleteUser: http.StatusForbidden},
	} {
		t.Run(c.builtin, func(t *testing.T) {
			w := &world{t: t, server: w.server, pool: w.pool}
			email, password := w.createStaff(c.builtin)
			if _, err := w.pool.Exec(context.Background(), `UPDATE app.users SET session_epoch = 3 WHERE lower(email) = lower($1)`, email); err != nil {
				t.Fatal(err)
			}
			browser := w.browser()
			_, epoch := browser.signIn(email, password)
			var stored int
			if err := w.pool.QueryRow(context.Background(), `SELECT session_epoch FROM app.users WHERE lower(email) = lower($1)`, email).Scan(&stored); err != nil {
				t.Fatal(err)
			}
			if epoch != stored {
				t.Errorf("the token carries epoch %d, the account %d", epoch, stored)
			}
			me := browser.must(http.StatusOK, http.MethodGet, "/auth/me", nil)
			var workspaces []string
			for _, workspace := range me["workspaces"].([]any) {
				workspaces = append(workspaces, workspace.(string))
			}
			if !slices.Equal(workspaces, c.workspaces) {
				t.Errorf("workspaces %v, want %v", workspaces, c.workspaces)
			}
			var manages bool
			for _, key := range me["permissions"].([]any) {
				manages = manages || key == "people.users.manage"
			}
			if manages != c.manageUsers {
				t.Errorf("people.users.manage granted: %v, want %v", manages, c.manageUsers)
			}
			browser.must(http.StatusOK, http.MethodGet, "/teacher/dashboard", nil)
			if status, body := browser.call(http.MethodDelete, "/admin/users/01935000-0000-7000-8000-00000000ffff", nil); status != c.deleteUser {
				t.Errorf("DELETE /admin/users/{id}: %d, want %d: %v", status, c.deleteUser, body)
			}
		})
	}
}
