//go:build e2e

package e2e

import (
	"net/http"
	"testing"
	"time"
)

func windowInput(versionID, classID string, opens, closes time.Time, mutate func(map[string]any)) map[string]any {
	in := map[string]any{
		"testVersionId":   versionID,
		"targets":         map[string]any{"classIds": []string{classID}, "studentIds": []string{}},
		"window":          map[string]any{"opensAt": rfc3339(opens), "closesAt": rfc3339(closes)},
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

func TestAClosedAssignmentIsOpenedAgainOnlyByReopeningIt(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)

	classID := teacher.class("Lớp đã đóng " + nonce(t))
	_, versionID := teacher.publishedVersion("Đề đã đóng " + nonce(t))

	now := time.Now()
	input := func(opens, closes time.Duration, mutate func(map[string]any)) map[string]any {
		return windowInput(versionID, classID, now.Add(opens), now.Add(closes), mutate)
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

func TestCloseNowBeforeTheWindowOpensIsA400AndACreateIgnoresIt(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)

	classID := teacher.class("Lớp đóng ngay " + nonce(t))
	_, versionID := teacher.publishedVersion("Đề đóng ngay " + nonce(t))
	now := time.Now()
	closeNow := func(in map[string]any) { in["closeNow"] = true }
	input := func(opens, closes time.Duration) map[string]any {
		return windowInput(versionID, classID, now.Add(opens), now.Add(closes), closeNow)
	}

	created := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/assignments", input(time.Hour, 2*time.Hour))
	if created["status"] != "scheduled" {
		t.Errorf("a create with closeNow and an opening ahead is %v, want scheduled and not closed", created["status"])
	}
	openCreated := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/assignments", input(-time.Hour, time.Hour))
	if openCreated["status"] != "open" {
		t.Errorf("a create with closeNow and an open window is %v, want open", openCreated["status"])
	}

	refuses := func(label, assignmentID string, body map[string]any) {
		t.Helper()
		status, answer := teacher.call(http.MethodPatch, "/teacher/assignments/"+assignmentID, body)
		details, _ := answer["error"].(map[string]any)["details"].(map[string]any)
		code, _ := answer["error"].(map[string]any)["code"].(string)
		if status != http.StatusBadRequest || code != "VALIDATION_FAILED" || details["closeNow"] == nil {
			t.Errorf("%s: status %d code %q details %v, want 400 VALIDATION_FAILED naming closeNow", label, status, code, details)
		}
	}
	refuses("a scheduled assignment closed now", id(created), input(time.Hour, 2*time.Hour))
	refuses("an open assignment closed now with its opening moved ahead", id(openCreated), input(30*time.Minute, 2*time.Hour))

	closedAlready := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/assignments", windowInput(versionID, classID, now.Add(-2*time.Hour), now.Add(-time.Hour), nil))
	refuses("a closed assignment closed now with its opening ahead", id(closedAlready), input(time.Hour, 2*time.Hour))

	for _, assignmentID := range []string{id(created), id(openCreated), id(closedAlready)} {
		after := teacher.must(http.StatusOK, http.MethodGet, "/teacher/assignments/"+assignmentID, nil)
		if after["status"] == "closed" && assignmentID != id(closedAlready) {
			t.Errorf("a refused save closed %s", assignmentID)
		}
	}

	closedNow := teacher.must(http.StatusOK, http.MethodPatch, "/teacher/assignments/"+id(openCreated), input(-time.Hour, time.Hour))
	if closedNow["status"] != "closed" {
		t.Errorf("closing an open assignment now: %v, want closed", closedNow["status"])
	}
}
