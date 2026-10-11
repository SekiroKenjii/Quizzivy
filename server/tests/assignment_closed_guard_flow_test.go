//go:build e2e

package e2e

import (
	"net/http"
	"testing"
	"time"
)

func TestAClosedAssignmentIsOpenedAgainOnlyByReopeningIt(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)

	classID := teacher.class("Lớp đã đóng " + nonce(t))
	_, versionID := teacher.publishedVersion("Đề đã đóng " + nonce(t))

	now := time.Now()
	input := func(opens, closes time.Duration, mutate func(map[string]any)) map[string]any {
		in := map[string]any{
			"testVersionId":   versionID,
			"targets":         map[string]any{"classIds": []string{classID}, "studentIds": []string{}},
			"window":          map[string]any{"opensAt": rfc3339(now.Add(opens)), "closesAt": rfc3339(now.Add(closes))},
			"durationMinutes": 30,
			"maxAttempts":     1,
			"review":          map[string]any{"showScore": true, "showCorrectAnswers": true, "showExplanations": true},
			"integrity": map[string]any{
				"requireFullscreen": false, "blockCopyPaste": true, "maxFocusLoss": 0, "onLimitExceeded": "flag", "minAwayMs": 3000,
			},
		}
		if mutate != nil {
			mutate(in)
		}
		return in
	}
	refused := func(assignmentID string, body map[string]any, status int, wantCode string) {
		t.Helper()
		got, answer := teacher.call(http.MethodPatch, "/teacher/assignments/"+assignmentID, body)
		code, _ := answer["error"].(map[string]any)["code"].(string)
		if got != status || code != wantCode {
			t.Errorf("PATCH %s: status %d code %q, want %d %s: %v", assignmentID, got, code, status, wantCode, answer)
		}
	}

	closed := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/assignments", input(-2*time.Hour, -time.Hour, nil))
	closedID := id(closed)
	if closed["status"] != "closed" {
		t.Fatalf("an assignment whose window has ended is %v, want closed", closed["status"])
	}

	refused(closedID, input(-2*time.Hour, time.Hour, nil), http.StatusConflict, "ASSIGNMENT_CLOSED")
	refused(closedID, input(time.Hour, 2*time.Hour, nil), http.StatusConflict, "ASSIGNMENT_CLOSED")
	after := teacher.must(http.StatusOK, http.MethodGet, "/teacher/assignments/"+closedID, nil)
	if after["status"] != "closed" || after["window"].(map[string]any)["closesAt"] != closed["window"].(map[string]any)["closesAt"] {
		t.Errorf("a refused save changed the assignment: %v", after)
	}

	teacher.must(http.StatusOK, http.MethodPatch, "/teacher/assignments/"+closedID, input(-2*time.Hour, -90*time.Minute, nil))
	noted := teacher.must(http.StatusOK, http.MethodPatch, "/teacher/assignments/"+closedID, input(-2*time.Hour, -90*time.Minute, func(in map[string]any) {
		in["studentNote"] = "Ghi chú sau khi đóng."
		in["review"].(map[string]any)["showScore"] = false
	}))
	if noted["status"] != "closed" || noted["studentNote"] != "Ghi chú sau khi đóng." {
		t.Errorf("editing the note of a closed assignment: %v", noted)
	}

	reopened := teacher.must(http.StatusOK, http.MethodPost, "/teacher/assignments/"+closedID+"/reopen", map[string]any{
		"closesAt": rfc3339(now.Add(time.Hour)), "reason": "Cả lớp mất điện.",
	})
	if reopened["status"] != "open" {
		t.Errorf("a reopened assignment is %v, want open", reopened["status"])
	}
	longer := teacher.must(http.StatusOK, http.MethodPatch, "/teacher/assignments/"+closedID, input(-2*time.Hour, 2*time.Hour, nil))
	if longer["status"] != "open" {
		t.Errorf("an open assignment given a later close is %v, want open", longer["status"])
	}

	early := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/assignments", input(-time.Hour, time.Hour, nil))
	earlyID := id(early)
	teacher.must(http.StatusOK, http.MethodPatch, "/teacher/assignments/"+earlyID, input(-time.Hour, time.Hour, func(in map[string]any) { in["closeNow"] = true }))
	later := teacher.must(http.StatusOK, http.MethodPatch, "/teacher/assignments/"+earlyID, input(-time.Hour, 3*time.Hour, nil))
	if later["status"] != "closed" {
		t.Errorf("a later close on an assignment closed early: %v, want it still closed", later["status"])
	}
	refused(earlyID, input(time.Hour, 3*time.Hour, nil), http.StatusConflict, "ASSIGNMENT_CLOSED")
}
