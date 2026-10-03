//go:build e2e

package e2e

import (
	"net/http"
	"strings"
	"testing"

	"quizzivy/internal/platform/config"
)

func TestAJoinCodeRedeemsAcrossAKeyRotationAndNotWithoutTheOldKey(t *testing.T) {
	oldKey := []byte(strings.Repeat("old-join-code-key-", 2)[:32])
	newKey := []byte(strings.Repeat("new-join-code-key-", 2)[:32])
	issuing := boot(t, func(c *config.Config) { c.JoinCodeKey = oldKey })
	email, password := issuing.teacher()
	teacher := issuing.browser()
	teacher.login(email, password)
	classID := teacher.class("Lớp xoay khoá " + nonce(t))
	code := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/classes/"+classID+"/join-code", map[string]any{})["code"].(string)

	rotating := boot(t, func(c *config.Config) { c.JoinCodeKey, c.JoinCodeKeyPrevious = newKey, oldKey })
	if preview := rotating.browser().must(http.StatusOK, http.MethodPost, "/join/preview", map[string]any{"joinCode": code}); preview["classId"] != classID {
		t.Fatalf("with the old key as previous, the preview names %v", preview)
	}

	rekeyed := boot(t, func(c *config.Config) { c.JoinCodeKey = newKey })
	status, body := rekeyed.browser().call(http.MethodPost, "/join/preview", map[string]any{"joinCode": code})
	if refusal, _ := body["error"].(map[string]any); status != http.StatusNotFound || refusal["code"] != "JOIN_CODE_INVALID" {
		t.Fatalf("with the old key gone, the preview answered %d %v, want 404 JOIN_CODE_INVALID", status, body)
	}
}

func TestATeacherReadsTheirJoinCodeBackAndNoOneElseCan(t *testing.T) {
	w := boot(t)
	email, password := w.createStaff("teacher")
	owner := w.browser()
	owner.login(email, password)
	classID := owner.class("Lớp đọc mã " + nonce(t))
	path := "/teacher/classes/" + classID + "/join-code"
	if status, _ := owner.call(http.MethodGet, path, nil); status != http.StatusNotFound {
		t.Fatalf("a class without a code answered %d, want 404", status)
	}
	issued := owner.must(http.StatusCreated, http.MethodPost, path, map[string]any{})

	status, body := owner.call(http.MethodGet, path, nil)
	if status != http.StatusOK || body["code"] != issued["code"] || body["legacy"] != false || body["hint"] == nil {
		t.Fatalf("the owner read %d %v, want the issued code %v", status, body, issued["code"])
	}

	otherEmail, otherPassword := w.createStaff("teacher")
	other := w.browser()
	other.login(otherEmail, otherPassword)
	if status, body := other.call(http.MethodGet, path, nil); status != http.StatusNotFound || body["code"] != nil {
		t.Errorf("another teacher read %d %v, want 404", status, body)
	}

	created := owner.must(http.StatusCreated, http.MethodPost, "/teacher/students", map[string]any{
		"email": "reader-" + nonce(t) + "@example.com", "fullName": "Học viên đọc mã", "classIds": []string{classID},
	})
	student := w.browser()
	student.login(created["user"].(map[string]any)["email"].(string), created["temporaryPassword"].(string))
	if status, body := student.call(http.MethodGet, path, nil); status != http.StatusForbidden || body["code"] != nil {
		t.Errorf("a student read %d %v, want 403", status, body)
	}
}
