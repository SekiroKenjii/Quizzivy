//go:build e2e

package e2e

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func errorCode(body map[string]any) any {
	refusal, _ := body["error"].(map[string]any)
	return refusal["code"]
}

func TestAnArchivedClassTakesNoNewJoinCode(t *testing.T) {
	w := boot(t)
	email, password := w.createStaff("teacher")
	owner := w.signedIn(email, password)
	otherEmail, otherPassword := w.createStaff("teacher")
	other := w.signedIn(otherEmail, otherPassword)
	adminEmail, adminPassword := w.createStaff("admin")
	admin := w.signedIn(adminEmail, adminPassword)

	classID := owner.class("Lớp lưu trữ " + nonce(t))
	path := "/teacher/classes/" + classID + "/join-code"
	first := owner.must(http.StatusCreated, http.MethodPost, path, map[string]any{})["code"].(string)

	owner.must(http.StatusOK, http.MethodPatch, "/teacher/classes/"+classID, map[string]any{"archived": true})

	status, refused := owner.call(http.MethodPost, path, map[string]any{})
	if status != http.StatusConflict || errorCode(refused) != "CLASS_ARCHIVED" {
		t.Fatalf("rotating the code of an archived class answered %d %v, want 409 CLASS_ARCHIVED", status, refused)
	}
	if kept := owner.must(http.StatusOK, http.MethodGet, path, nil); kept["code"] != first {
		t.Errorf("the refused rotation changed the class's code from %q to %v", first, kept["code"])
	}

	if status, body := admin.call(http.MethodPost, path, map[string]any{}); status != http.StatusConflict || errorCode(body) != "CLASS_ARCHIVED" {
		t.Errorf("an Admin rotating the code of an archived class answered %d %v, want 409 CLASS_ARCHIVED", status, body)
	}

	body := new(jsonPayload(t, map[string]any{}))
	theirs := other.send(http.MethodPost, path, body)
	missing := other.send(http.MethodPost, "/teacher/classes/"+uuid.NewString()+"/join-code", body)
	if theirs.status != http.StatusNotFound || answer(theirs) != answer(missing) {
		t.Errorf("another teacher rotating an archived class answered\n  %s\nwhere a missing class answers\n  %s", answer(theirs), answer(missing))
	}
	if kept := owner.must(http.StatusOK, http.MethodGet, path, nil); kept["code"] != first {
		t.Errorf("another teacher's refused rotation changed the class's code from %q to %v", first, kept["code"])
	}

	status, preview := w.browser().call(http.MethodPost, "/join/preview", map[string]any{"joinCode": first})
	if status != http.StatusNotFound || errorCode(preview) != "JOIN_CODE_INVALID" {
		t.Errorf("previewing the code of an archived class answered %d %v, want 404 JOIN_CODE_INVALID", status, preview)
	}

	owner.must(http.StatusNoContent, http.MethodDelete, path, nil)
	if status, _ := owner.call(http.MethodGet, path, nil); status != http.StatusNotFound {
		t.Errorf("after revoking the code of an archived class, reading it answered %d, want 404", status)
	}

	owner.must(http.StatusOK, http.MethodPatch, "/teacher/classes/"+classID, map[string]any{"archived": false})
	issued := owner.must(http.StatusCreated, http.MethodPost, path, map[string]any{})
	if issued["code"] == first {
		t.Error("the restored class was issued the code it had before")
	}
	if kept := owner.must(http.StatusOK, http.MethodGet, path, nil); kept["code"] != issued["code"] {
		t.Errorf("the restored class reads back %v, want the code it was issued, %v", kept["code"], issued["code"])
	}
}
