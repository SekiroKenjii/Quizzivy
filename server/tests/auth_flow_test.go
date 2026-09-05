//go:build e2e

package e2e

import (
	"net/http"
	"testing"
)

func TestATeacherSignsInRefreshesAndSignsOut(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	browser := w.browser()

	first := browser.login(email, password)
	if first["user"].(map[string]any)["email"] != email {
		t.Fatalf("login answered for someone else: %v", first["user"])
	}

	me := browser.must(http.StatusOK, http.MethodGet, "/auth/me", nil)
	if me["role"] != "admin" {
		t.Fatalf("/auth/me = %v", me)
	}

	refreshed := browser.must(http.StatusOK, http.MethodPost, "/auth/refresh", nil)
	if token, _ := refreshed["accessToken"].(string); token == "" {
		t.Fatalf("refresh did not mint an access token: %v", refreshed)
	}

	browser.must(http.StatusNoContent, http.MethodPost, "/auth/logout", nil)
	if status, _ := browser.call(http.MethodPost, "/auth/refresh", nil); status != http.StatusUnauthorized {
		t.Fatalf("refresh after logout answered %d, want 401", status)
	}
}
