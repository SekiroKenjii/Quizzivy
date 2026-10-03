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
