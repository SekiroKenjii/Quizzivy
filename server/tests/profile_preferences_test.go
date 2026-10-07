//go:build e2e

package e2e

import (
	"net/http"
	"testing"
)

func TestProfilePreferencesAreSelfScopedAndReturnPersistedMergedValues(t *testing.T) {
	w := boot(t)
	aEmail, aPassword := w.createStaff("teacher")
	bEmail, bPassword := w.createStaff("teacher")
	a, b := w.signedIn(aEmail, aPassword), w.signedIn(bEmail, bPassword)
	aBefore := a.must(http.StatusOK, http.MethodGet, "/auth/me", nil)
	bBefore := b.must(http.StatusOK, http.MethodGet, "/auth/me", nil)
	changed := a.must(http.StatusOK, http.MethodPatch, "/auth/me", map[string]any{"displayName": "  Cô An  ", "phone": "+84 123456", "locale": "en", "timeZone": "UTC"})
	if changed["id"] != aBefore["id"] || changed["displayName"] != "Cô An" || changed["phone"] != "+84 123456" || changed["fullName"] != aBefore["fullName"] {
		t.Fatalf("profile=%v", changed)
	}
	a.must(http.StatusOK, http.MethodPatch, "/me/preferences", map[string]any{"theme": "dark", "compactTables": true, "assignmentDefaults": map[string]any{"durationMinutes": 45, "showScore": true}})
	merged := a.must(http.StatusOK, http.MethodPatch, "/me/preferences", map[string]any{"compactTables": false, "assignmentDefaults": map[string]any{"showScore": false}})
	nested, ok := merged["assignmentDefaults"].(map[string]any)
	if !ok || merged["theme"] != "dark" || merged["compactTables"] != false || len(nested) != 1 || nested["showScore"] != false {
		t.Fatalf("merged=%v", merged)
	}
	a.must(http.StatusOK, http.MethodPatch, "/me/preferences", map[string]any{})
	a.must(http.StatusBadRequest, http.MethodPatch, "/me/preferences", map[string]any{"userId": bBefore["id"], "theme": "light"})
	a.must(http.StatusBadRequest, http.MethodPatch, "/auth/me", map[string]any{"id": bBefore["id"], "displayName": "Forged"})
	a.must(http.StatusBadRequest, http.MethodPatch, "/auth/me", map[string]any{"timeZone": "Local"})
	cleared := a.must(http.StatusOK, http.MethodPatch, "/auth/me", map[string]any{"displayName": nil, "phone": nil})
	if _, ok := cleared["phone"]; ok {
		t.Fatal("phone clear serialized a value")
	}
	if _, ok := cleared["displayName"]; ok {
		t.Fatal("display name clear serialized a value")
	}
	bAfter := b.must(http.StatusOK, http.MethodGet, "/auth/me", nil)
	if bAfter["id"] != bBefore["id"] || bAfter["fullName"] != bBefore["fullName"] {
		t.Fatal("another actor changed")
	}
	if _, ok := bAfter["phone"]; ok {
		t.Fatal("another actor received private phone")
	}
	if _, ok := bAfter["timeZone"]; ok {
		t.Fatal("another actor inherited zone")
	}
}
