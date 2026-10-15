//go:build e2e

package e2e

import (
	"net/http"
	"slices"
	"testing"
)

func TestATeacherSignsInRefreshesAndSignsOut(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	browser := w.browser()

	first := browser.login(email, password)
	user := first["user"].(map[string]any)
	if user["email"] != email {
		t.Fatalf("login answered for someone else: %v", first["user"])
	}
	requireAdminRoleRecord(t, "login", user)

	me := browser.must(http.StatusOK, http.MethodGet, "/auth/me", nil)
	workspaces, _ := me["workspaces"].([]any)
	if me["email"] != email || !slices.Contains(workspaces, any("teacher")) {
		t.Fatalf("/auth/me = %v", me)
	}
	requireAdminRoleRecord(t, "/auth/me", me)

	refreshed := browser.must(http.StatusOK, http.MethodPost, "/auth/refresh", nil)
	if token, _ := refreshed["accessToken"].(string); token == "" {
		t.Fatalf("refresh did not mint an access token: %v", refreshed)
	}

	browser.must(http.StatusNoContent, http.MethodPost, "/auth/logout", nil)
	if status, _ := browser.call(http.MethodPost, "/auth/refresh", nil); status != http.StatusUnauthorized {
		t.Fatalf("refresh after logout answered %d, want 401", status)
	}
}

func requireAdminRoleRecord(t *testing.T, where string, user map[string]any) {
	t.Helper()
	role, ok := user["role"].(map[string]any)
	id, _ := role["id"].(string)
	if !ok || id == "" || role["builtinKey"] != "admin" || role["name"] != "Quản trị viên" || len(role) != 3 {
		t.Fatalf("%s: role = %v, want the built-in Admin's record and nothing but id, builtinKey and name", where, user["role"])
	}
}
