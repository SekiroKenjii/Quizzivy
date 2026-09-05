//go:build e2e

package e2e

import (
	"net/http"
	"testing"
)

func TestAStudentJoinsByCodeSitsTheTestAndTheTeacherSeesIt(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)

	classID := teacher.class("Lớp mã " + nonce(t))
	issued := teacher.must(http.StatusCreated, http.MethodPost, "/admin/classes/"+classID+"/join-code", map[string]any{})
	code := issued["code"].(string)

	anonymous := w.browser()
	preview := anonymous.must(http.StatusOK, http.MethodPost, "/join/preview", map[string]any{"joinCode": code})
	if preview["classId"] != classID {
		t.Fatalf("preview names %v, want the class the code was issued for", preview)
	}

	created := teacher.must(http.StatusCreated, http.MethodPost, "/admin/students", map[string]any{
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
	if questions, _ := session["questions"].([]any); len(questions) != 1 {
		t.Fatalf("the paper has %d questions, want 1", len(questions))
	}
	submitted := student.must(http.StatusOK, http.MethodPost, "/app/attempts/"+id(attempt)+"/submit",
		map[string]any{"sessionId": session["sessionId"], "reason": "manual"})
	if submitted["status"] != "graded" && submitted["status"] != "submitted" {
		t.Fatalf("after submitting the attempt is %v", submitted["status"])
	}
	student.must(http.StatusOK, http.MethodGet, "/app/attempts/"+id(attempt)+"/result", nil)

	monitor := teacher.must(http.StatusOK, http.MethodGet, "/admin/assignments/"+id(assignment)+"/attempts", nil)
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
