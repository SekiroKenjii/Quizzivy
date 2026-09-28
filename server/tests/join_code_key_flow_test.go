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
