//go:build e2e

package e2e

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAStudentJoinsByCodeSitsTheTestAndTheTeacherSeesIt(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)

	classID := teacher.class("Lớp mã " + nonce(t))
	issued := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/classes/"+classID+"/join-code", map[string]any{})
	code := issued["code"].(string)

	anonymous := w.browser()
	preview := anonymous.must(http.StatusOK, http.MethodPost, "/join/preview", map[string]any{"joinCode": code})
	if preview["classId"] != classID {
		t.Fatalf("preview names %v, want the class the code was issued for", preview)
	}

	created := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/students", map[string]any{
		"email": "student-" + nonce(t) + "@example.com", "fullName": "Nguyễn Văn An", "classIds": []string{},
	})
	studentEmail := created["user"].(map[string]any)["email"].(string)

	student := w.browser()
	student.login(studentEmail, created["temporaryPassword"].(string))
	joined := student.must(http.StatusOK, http.MethodPost, "/app/classes/join", map[string]any{"joinCode": code})
	if joined["id"] != classID {
		t.Fatalf("joining answered %v", joined)
	}
	mine := student.must(http.StatusOK, http.MethodGet, "/app/classes", nil)
	if items, _ := mine["items"].([]any); len(items) != 1 || items[0].(map[string]any)["id"] != classID {
		t.Fatalf("/app/classes = %v, want the one class just joined", mine)
	}

	_, versionID := teacher.publishedVersion("Kiểm tra vào lớp " + nonce(t))
	assignment := teacher.assign(versionID, classID)

	home := student.must(http.StatusOK, http.MethodGet, "/app/assignments", nil)
	due, _ := home["dueNow"].([]any)
	if len(due) != 1 || due[0].(map[string]any)["id"] != assignment["id"] {
		t.Fatalf("the student's home does not show the assignment as due: %v", home)
	}

	session := student.must(http.StatusOK, http.MethodPost, "/app/assignments/"+id(assignment)+"/attempts", nil)
	attempt := session["attempt"].(map[string]any)
	questions, _ := session["questions"].([]any)
	if len(questions) != 1 {
		t.Fatalf("the paper has %d questions, want 1", len(questions))
	}
	resuming := student.must(http.StatusOK, http.MethodGet, "/app/assignments", nil)
	live, _ := resuming["dueNow"].([]any)
	if len(live) != 1 || live[0].(map[string]any)["liveAnsweredCount"] != float64(0) {
		t.Fatalf("the card of a paper just started does not count 0 answers: %v", resuming)
	}
	submitted := student.must(http.StatusOK, http.MethodPost, "/app/attempts/"+id(attempt)+"/submit",
		map[string]any{"sessionId": session["sessionId"], "reason": "manual"})
	if submitted["status"] != "graded" && submitted["status"] != "submitted" {
		t.Fatalf("after submitting the attempt is %v", submitted["status"])
	}
	result := student.must(http.StatusOK, http.MethodGet, "/app/attempts/"+id(attempt)+"/result", nil)
	parts, _ := result["sections"].([]any)
	marked, _ := result["questions"].([]any)
	if len(parts) != 1 || len(marked) != 1 {
		t.Fatalf("the result has %d parts and %d questions, want 1 and 1", len(parts), len(marked))
	}
	part := parts[0].(map[string]any)["id"]
	if part != questions[0].(map[string]any)["sectionId"] || marked[0].(map[string]any)["sectionId"] != part {
		t.Fatalf("the result puts its question in %v of %v; the paper showed %v",
			marked[0].(map[string]any)["sectionId"], parts, questions[0].(map[string]any)["sectionId"])
	}
	finished := student.must(http.StatusOK, http.MethodGet, "/app/assignments", nil)
	for _, list := range []string{"dueNow", "completed"} {
		cards, _ := finished[list].([]any)
		for _, card := range cards {
			if count, present := card.(map[string]any)["liveAnsweredCount"]; present {
				t.Fatalf("a submitted paper still counts %v live answers: %v", count, card)
			}
		}
	}

	monitor := teacher.must(http.StatusOK, http.MethodGet, "/teacher/assignments/"+id(assignment)+"/attempts", nil)
	rows, _ := monitor["rows"].([]any)
	seen := false
	for _, row := range rows {
		if row.(map[string]any)["attemptId"] == id(attempt) {
			seen = true
		}
	}
	if !seen {
		t.Fatalf("the monitor does not show the submitted attempt: %v", monitor)
	}
}

func closedAssignment(teacher *client, versionID, classID string) map[string]any {
	now := time.Now()
	return teacher.must(http.StatusCreated, http.MethodPost, "/teacher/assignments", map[string]any{
		"testVersionId":   versionID,
		"targets":         map[string]any{"classIds": []string{classID}, "studentIds": []string{}},
		"window":          map[string]any{"opensAt": rfc3339(now.Add(-3 * time.Hour)), "closesAt": rfc3339(now.Add(-time.Hour))},
		"durationMinutes": 30,
		"maxAttempts":     1,
		"review":          map[string]any{"showScore": true, "showCorrectAnswers": false, "showExplanations": false},
		"integrity": map[string]any{
			"requireFullscreen": false, "blockCopyPaste": true, "maxFocusLoss": 0, "onLimitExceeded": "flag", "minAwayMs": 3000,
		},
	})
}

func TestAClosedAssignmentIsReopenedForOneStudentAlone(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)

	classID := teacher.class("Lớp mở lại " + nonce(t))
	_, versionID := teacher.publishedVersion("Đề mở lại " + nonce(t))
	enrol := func(name string) (*client, string) {
		created := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/students", map[string]any{
			"email": "hoc-vien-" + nonce(t) + "@example.com", "fullName": name, "classIds": []string{classID},
		})
		user := created["user"].(map[string]any)
		student := w.browser()
		student.login(user["email"].(string), created["temporaryPassword"].(string))
		return student, user["id"].(string)
	}
	reopened, reopenedID := enrol("An mở lại")
	other, _ := enrol("Bình đóng")
	assignment := closedAssignment(teacher, versionID, classID)
	path := "/teacher/assignments/" + id(assignment) + "/student-overrides"

	refused := func(student *client) {
		t.Helper()
		status, body := student.call(http.MethodPost, "/app/assignments/"+id(assignment)+"/attempts", nil)
		if code, _ := body["error"].(map[string]any)["code"].(string); status != http.StatusConflict || code != "ASSIGNMENT_NOT_OPEN" {
			t.Fatalf("starting a closed assignment answered %d %v, want 409 ASSIGNMENT_NOT_OPEN", status, body)
		}
	}
	refused(reopened)
	refused(other)

	status, body := teacher.call(http.MethodPost, "/teacher/assignments/"+id(assignment)+"/extend", map[string]any{"minutes": 30})
	if code, _ := body["error"].(map[string]any)["code"].(string); status != http.StatusConflict || code != "ASSIGNMENT_CLOSED" {
		t.Fatalf("extending a closed assignment answered %d %v, want 409 ASSIGNMENT_CLOSED", status, body)
	}
	status, body = teacher.call(http.MethodPut, path, map[string]any{"studentIds": []string{reopenedID}, "extendBy": 30, "reason": "Mở lại cho em"})
	if status != http.StatusConflict {
		t.Fatalf("extending a student whose close has passed answered %d %v, want 409", status, body)
	}

	const reason = "Mất điện trong giờ làm bài"
	until := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	set := teacher.must(http.StatusOK, http.MethodPut, path, map[string]any{
		"studentIds": []string{reopenedID}, "closesAt": rfc3339(until), "durationMinutes": 20, "reason": reason, "notify": true,
	})
	if items, _ := set["items"].([]any); len(items) != 1 || items[0].(map[string]any)["studentId"] != reopenedID || items[0].(map[string]any)["reason"] != reason {
		t.Fatalf("setting the override answered %v", set)
	}

	home := reopened.must(http.StatusOK, http.MethodGet, "/app/assignments", nil)
	due, _ := home["dueNow"].([]any)
	if len(due) != 1 || due[0].(map[string]any)["id"] != id(assignment) || due[0].(map[string]any)["status"] != "open" {
		t.Fatalf("the reopened student's home = %v, want the assignment open and due", home)
	}
	if got, err := time.Parse(time.RFC3339Nano, due[0].(map[string]any)["closesAt"].(string)); err != nil || !got.Equal(until) {
		t.Errorf("the card closes at %v (%v), want the student's own %v", got, err, until)
	}
	intro := reopened.must(http.StatusOK, http.MethodGet, "/app/assignments/"+id(assignment), nil)
	if intro["status"] != "open" || intro["durationMinutes"] != float64(20) {
		t.Errorf("the intro says %v for %v minutes, want open for 20", intro["status"], intro["durationMinutes"])
	}
	for label, body := range map[string]map[string]any{"home": home, "intro": intro} {
		keys := map[string]bool{}
		keysOf(body, keys)
		for _, teacherOnly := range []string{"reason", "extraAttempts", "studentName", "extendBy", "notify"} {
			if keys[teacherOnly] {
				t.Errorf("the student's %s carries the teacher's %q", label, teacherOnly)
			}
		}
		if raw := jsonNumber(body); strings.Contains(raw, reason) {
			t.Errorf("the student's %s carries the teacher's reason", label)
		}
	}

	untouched := other.must(http.StatusOK, http.MethodGet, "/app/assignments", nil)
	if cards, _ := untouched["dueNow"].([]any); len(cards) != 0 {
		t.Errorf("the classmate's home = %v, want nothing due", untouched)
	}
	if closed := other.must(http.StatusOK, http.MethodGet, "/app/assignments/"+id(assignment), nil); closed["status"] != "closed" {
		t.Errorf("the classmate's intro says %v, want closed", closed["status"])
	}
	refused(other)

	session := reopened.must(http.StatusOK, http.MethodPost, "/app/assignments/"+id(assignment)+"/attempts", nil)
	attempt := session["attempt"].(map[string]any)
	started, _ := time.Parse(time.RFC3339Nano, attempt["startedAt"].(string))
	deadline, _ := time.Parse(time.RFC3339Nano, attempt["deadlineAt"].(string))
	if d := deadline.Sub(started); d < 19*time.Minute || d > 21*time.Minute {
		t.Errorf("the reopened student's attempt runs %v, want their 20 minutes", d)
	}

	listed := teacher.must(http.StatusOK, http.MethodGet, path, nil)
	if items, _ := listed["items"].([]any); len(items) != 1 {
		t.Fatalf("the teacher lists %v, want the one override", listed)
	}
	teacher.must(http.StatusNoContent, http.MethodDelete, path+"/"+reopenedID, nil)
	if status, _ := teacher.call(http.MethodDelete, path+"/"+reopenedID, nil); status != http.StatusNotFound {
		t.Errorf("removing it twice answered %d, want 404", status)
	}
	if listed := teacher.must(http.StatusOK, http.MethodGet, path, nil); len(listed["items"].([]any)) != 0 {
		t.Errorf("after removal the teacher lists %v", listed)
	}
}

func TestExtendingAnOpenAssignmentMovesTheCloseForEveryone(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)

	classID := teacher.class("Lớp gia hạn " + nonce(t))
	_, versionID := teacher.publishedVersion("Đề gia hạn " + nonce(t))
	assignment := teacher.assign(versionID, classID)
	before, _ := time.Parse(time.RFC3339, assignment["window"].(map[string]any)["closesAt"].(string))

	extended := teacher.must(http.StatusOK, http.MethodPost, "/teacher/assignments/"+id(assignment)+"/extend", map[string]any{"minutes": 45, "notify": true})
	after, _ := time.Parse(time.RFC3339, extended["window"].(map[string]any)["closesAt"].(string))
	if after.Sub(before) != 45*time.Minute {
		t.Errorf("the close moved by %v, want 45m", after.Sub(before))
	}

	for label, minutes := range map[string]int{"zero": 0, "past a week": 10081} {
		if status, _ := teacher.call(http.MethodPost, "/teacher/assignments/"+id(assignment)+"/extend", map[string]any{"minutes": minutes}); status != http.StatusBadRequest {
			t.Errorf("extending by %s answered %d, want 400", label, status)
		}
	}
	other := w.browser()
	otherEmail, otherPassword := w.createStaff("teacher")
	other.login(otherEmail, otherPassword)
	if status, _ := other.call(http.MethodPost, "/teacher/assignments/"+id(assignment)+"/extend", map[string]any{"minutes": 5}); status != http.StatusNotFound {
		t.Errorf("another teacher extending it answered %d, want 404", status)
	}
}
