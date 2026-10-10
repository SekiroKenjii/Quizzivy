//go:build e2e

package e2e

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func (c *client) inbox(t *testing.T) []map[string]any {
	t.Helper()
	page := c.must(http.StatusOK, http.MethodGet, "/me/notifications", nil)
	var items []map[string]any
	for _, item := range page["items"].([]any) {
		items = append(items, item.(map[string]any))
	}
	return items
}

func ofKind(items []map[string]any, kind string) []map[string]any {
	var out []map[string]any
	for _, item := range items {
		if item["kind"] == kind {
			out = append(out, item)
		}
	}
	return out
}

func windowed(c *client, versionID, classID string, opens, closes time.Time) map[string]any {
	return c.must(http.StatusCreated, http.MethodPost, "/teacher/assignments", map[string]any{
		"testVersionId":   versionID,
		"targets":         map[string]any{"classIds": []string{classID}, "studentIds": []string{}},
		"window":          map[string]any{"opensAt": rfc3339(opens), "closesAt": rfc3339(closes)},
		"durationMinutes": 30,
		"maxAttempts":     1,
		"review":          map[string]any{"showScore": true, "showCorrectAnswers": false, "showExplanations": false},
		"integrity": map[string]any{
			"requireFullscreen": false, "blockCopyPaste": true, "maxFocusLoss": 0, "onLimitExceeded": "flag", "minAwayMs": 3000,
		},
	})
}

func TestTheNoticesTheProductMakesReachTheRightPeopleAndNoOneElse(t *testing.T) {
	w := boot(t)
	teacherEmail, teacherPassword := w.createStaff("teacher")
	strangerEmail, strangerPassword := w.createStaff("teacher")
	teacher, stranger := w.signedIn(teacherEmail, teacherPassword), w.signedIn(strangerEmail, strangerPassword)

	classID := teacher.class("Lớp thông báo " + nonce(t))
	code := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/classes/"+classID+"/join-code", map[string]any{})["code"].(string)
	created := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/students", map[string]any{
		"email": "thong-bao-" + nonce(t) + "@example.com", "fullName": "Nguyễn Văn An", "classIds": []string{},
	})
	student := w.browser()
	student.login(created["user"].(map[string]any)["email"].(string), created["temporaryPassword"].(string))
	student.must(http.StatusOK, http.MethodPost, "/app/classes/join", map[string]any{"joinCode": code})

	_, versionID := teacher.publishedVersion("Kiểm tra thông báo " + nonce(t))
	now := time.Now()
	open := windowed(teacher, versionID, classID, now.Add(-time.Minute), now.Add(2*time.Hour))
	closing := windowed(teacher, versionID, classID, now.Add(-2*time.Hour), now.Add(30*time.Minute))

	if got := student.unread(t); got != 2 {
		t.Fatalf("the student has %v unread, want the two assignments that opened", got)
	}
	opened := ofKind(student.inbox(t), "assignment.opened")
	if len(opened) != 2 {
		t.Fatalf("the student's notices are %v, want two assignment.opened", opened)
	}
	for _, n := range opened {
		target := n["target"].(map[string]any)
		if target["route"] != "studentAssignment" || (target["assignmentId"] != id(open) && target["assignmentId"] != id(closing)) {
			t.Errorf("the opening notice targets %v", target)
		}
		if _, err := time.Parse(time.RFC3339Nano, n["params"].(map[string]any)["closesAt"].(string)); err != nil {
			t.Errorf("the opening notice names no close: %v", n["params"])
		}
	}

	if got := teacher.unread(t); got != 2 {
		t.Fatalf("the teacher has %v unread, want the join and the assignment about to close", got)
	}
	teacherItems := teacher.inbox(t)
	if joined := ofKind(teacherItems, "class.joined"); len(joined) != 1 || joined[0]["params"].(map[string]any)["studentName"] != "Nguyễn Văn An" ||
		joined[0]["target"].(map[string]any)["route"] != "classes" {
		t.Errorf("the join notice is %v", joined)
	}
	if soon := ofKind(teacherItems, "assignment.closing"); len(soon) != 1 || soon[0]["params"].(map[string]any)["notSubmitted"] != float64(1) ||
		soon[0]["target"].(map[string]any)["assignmentId"] != id(closing) {
		t.Errorf("the closing notice is %v", soon)
	}

	session := student.must(http.StatusOK, http.MethodPost, "/app/assignments/"+id(open)+"/attempts", nil)
	student.must(http.StatusOK, http.MethodPost, "/app/attempts/"+id(session["attempt"].(map[string]any))+"/submit",
		map[string]any{"sessionId": session["sessionId"], "reason": "manual"})
	if got := teacher.unread(t); got != 3 {
		t.Errorf("after a paper came in the teacher has %v unread, want 3", got)
	}
	submitted := ofKind(teacher.inbox(t), "attempt.submitted")
	if len(submitted) != 1 || submitted[0]["params"].(map[string]any)["count"] != float64(1) ||
		submitted[0]["target"].(map[string]any)["assignmentId"] != id(open) {
		t.Fatalf("the submission notice is %v", submitted)
	}

	if got := stranger.unread(t); got != 0 {
		t.Errorf("a teacher with no class and no assignment has %v unread", got)
	}
	if items := stranger.inbox(t); len(items) != 0 {
		t.Errorf("a stranger is shown %v", items)
	}

	before := student.unread(t)
	teacher.must(http.StatusOK, http.MethodPost, "/teacher/assignments/"+id(open)+"/extend", map[string]any{"minutes": 30})
	if got := student.unread(t); got != before {
		t.Errorf("an extension nobody asked to announce moved the student's unread from %v to %v", before, got)
	}
	extended := teacher.must(http.StatusOK, http.MethodPost, "/teacher/assignments/"+id(open)+"/extend", map[string]any{"minutes": 30, "notify": true})
	if got := student.unread(t); got != before+1 {
		t.Errorf("an extension the teacher asked to announce left the student with %v unread, want %v", got, before+1)
	}
	told := ofKind(student.inbox(t), "assignment.extended")
	if len(told) != 1 || told[0]["params"].(map[string]any)["closesAt"] == nil {
		t.Fatalf("the extension notice is %v", told)
	}
	got, err := time.Parse(time.RFC3339Nano, told[0]["params"].(map[string]any)["closesAt"].(string))
	want, wantErr := time.Parse(time.RFC3339Nano, extended["window"].(map[string]any)["closesAt"].(string))
	if err != nil || wantErr != nil || !got.Equal(want) {
		t.Errorf("the student is told the close is %v, the assignment says %v", got, want)
	}
	for _, n := range student.inbox(t) {
		raw := jsonNumber(n)
		for _, leak := range []string{"score", "isCorrect", "sampleAnswer", "acceptedAnswers", "transcript"} {
			if strings.Contains(raw, leak) {
				t.Errorf("the student's notice %v carries %q", n["kind"], leak)
			}
		}
	}
}
