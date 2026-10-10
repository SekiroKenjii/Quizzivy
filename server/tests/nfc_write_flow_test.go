//go:build e2e

package e2e

import (
	"net/http"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"
)

func decomposedText(s string) string { return norm.NFD.String(s) }

func requireComposed(t *testing.T, what string, got any, want string) {
	t.Helper()
	text, _ := got.(string)
	if text != want || text != norm.NFC.String(text) {
		t.Fatalf("%s = %q, want the composed %q", what, got, want)
	}
}

func proseNode(text string) map[string]any {
	return map[string]any{"format": "semantic_v1", "blocks": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": text, "marks": []any{}}}}}}
}

func firstText(t *testing.T, document any) string {
	t.Helper()
	blocks := document.(map[string]any)["blocks"].([]any)
	return blocks[0].(map[string]any)["content"].([]any)[0].(map[string]any)["text"].(string)
}

func TestWhatATeacherTypesDecomposedIsStoredAndAnsweredComposed(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)
	d := decomposedText

	question := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/questions", map[string]any{
		"type": "short_answer", "points": 1, "prompt": d("Mô tả bức tranh"), "explanation": d("Vì sao như vậy"), "sampleAnswer": d("Mẫu trả lời"),
		"tags": []string{d("nghé"), "nghé", "shared"},
	})
	requireComposed(t, "question prompt", question["prompt"], "Mô tả bức tranh")
	requireComposed(t, "question explanation", question["explanation"], "Vì sao như vậy")
	requireComposed(t, "question sample answer", question["sampleAnswer"], "Mẫu trả lời")
	if tags := question["tags"].([]any); len(tags) != 3 || tags[0] != "nghé" || tags[1] != "nghé" {
		t.Fatalf("tags=%v", tags)
	}
	stored := teacher.must(http.StatusOK, http.MethodGet, "/teacher/questions/"+id(question), nil)
	requireComposed(t, "stored question prompt", stored["prompt"], "Mô tả bức tranh")

	text := d("Đọc kỹ đoạn văn")
	document := proseNode(text)
	rich := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/questions", map[string]any{
		"type": "short_answer", "points": 1, "prompt": text, "promptContent": document, "explanation": d("Giải thích"), "explanationContent": proseNode(d("Giải thích")),
	})
	requireComposed(t, "rich prompt", rich["prompt"], "Đọc kỹ đoạn văn")
	requireComposed(t, "rich prompt document", firstText(t, rich["promptContent"]), "Đọc kỹ đoạn văn")
	requireComposed(t, "rich explanation document", firstText(t, rich["explanationContent"]), "Giải thích")

	class := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/classes", map[string]any{"name": d("Lớp 10A"), "description": d("Lớp buổi tối")})
	requireComposed(t, "class name", class["name"], "Lớp 10A")
	requireComposed(t, "class description", class["description"], "Lớp buổi tối")
	renamed := teacher.must(http.StatusOK, http.MethodPatch, "/teacher/classes/"+id(class), map[string]any{"name": d("Lớp 10B")})
	requireComposed(t, "renamed class", renamed["name"], "Lớp 10B")
	requireComposed(t, "kept description", renamed["description"], "Lớp buổi tối")

	test := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/tests", map[string]any{"title": d("Đề kiểm tra"), "description": d("Mô tả đề")})
	requireComposed(t, "test title", test["title"], "Đề kiểm tra")
	requireComposed(t, "test description", test["description"], "Mô tả đề")
	outlined := teacher.must(http.StatusOK, http.MethodPatch, "/teacher/tests/"+id(test), map[string]any{
		"expectedUpdatedAt": test["updatedAt"],
		"title":             d("Đề kiểm tra giữa kỳ"),
		"sections":          []any{map[string]any{"title": d("Phần nghe"), "instructions": d("Làm bài cẩn thận"), "questionIds": []string{id(question)}}},
	})
	requireComposed(t, "renamed test", outlined["title"], "Đề kiểm tra giữa kỳ")
	section := outlined["sections"].([]any)[0].(map[string]any)
	requireComposed(t, "section title", section["title"], "Phần nghe")
	requireComposed(t, "section instructions", section["instructions"], "Làm bài cẩn thận")

	version := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/tests/"+id(test)+"/publish", map[string]any{"changeNote": "  " + d("Sửa đề lần một") + "  "})
	requireComposed(t, "change note", version["changeNote"], "Sửa đề lần một")

	student := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/students", map[string]any{
		"email": d("hoc.sinh.é") + "-" + nonce(t) + "@example.com", "fullName": d("Nguyễn Văn Á"), "classIds": []string{id(class)},
	})
	user := student["user"].(map[string]any)
	requireComposed(t, "student name", user["fullName"], "Nguyễn Văn Á")
	if email := user["email"].(string); !strings.HasPrefix(email, d("hoc.sinh.é")) {
		t.Fatalf("the email was rewritten: %q", email)
	}
	edited := teacher.must(http.StatusOK, http.MethodPatch, "/teacher/students/"+id(user), map[string]any{"fullName": d("Trần Thị B")})
	requireComposed(t, "edited student name", edited["fullName"], "Trần Thị B")

	assignment := teacher.assign(id(version), id(class))
	noted := teacher.must(http.StatusOK, http.MethodPatch, "/teacher/assignments/"+id(assignment), map[string]any{
		"testVersionId": assignment["testVersionId"], "targets": map[string]any{"classIds": []string{id(class)}, "studentIds": []string{}},
		"window": assignment["window"], "durationMinutes": assignment["durationMinutes"], "maxAttempts": assignment["maxAttempts"],
		"review": assignment["review"], "integrity": assignment["integrity"], "studentNote": d("Làm bài cẩn thận"),
	})
	requireComposed(t, "student note", noted["studentNote"], "Làm bài cẩn thận")
	overrides := teacher.must(http.StatusOK, http.MethodPut, "/teacher/assignments/"+id(assignment)+"/student-overrides", map[string]any{
		"studentIds": []string{id(user)}, "extraAttempts": 1, "reason": d("Máy bị hỏng"),
	})
	requireComposed(t, "override reason", overrides["items"].([]any)[0].(map[string]any)["reason"], "Máy bị hỏng")
}

func TestWhatAStudentTypesDecomposedIsSavedComposedAndGradedAsBefore(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)
	d := decomposedText

	question := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/questions", map[string]any{
		"type": "fill_blank", "points": 1, "prompt": "Thủ đô của Việt Nam là {{1}}",
		"blanks": []any{map[string]any{"ordinal": 1, "acceptedAnswers": []string{"Hà Nội"}}},
	})
	test := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/tests", map[string]any{"title": "Địa lý " + nonce(t)})
	teacher.must(http.StatusOK, http.MethodPatch, "/teacher/tests/"+id(test), map[string]any{
		"expectedUpdatedAt": test["updatedAt"], "sections": []any{map[string]any{"title": "Phần 1", "questionIds": []string{id(question)}}},
	})
	version := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/tests/"+id(test)+"/publish", nil)
	classID := teacher.class("Lớp địa " + nonce(t))
	created := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/students", map[string]any{
		"email": "dia-" + nonce(t) + "@example.com", "fullName": "Học viên", "classIds": []string{classID},
	})
	student := w.browser()
	student.login(created["user"].(map[string]any)["email"].(string), created["temporaryPassword"].(string))
	assignment := teacher.assign(id(version), classID)

	session := student.must(http.StatusOK, http.MethodPost, "/app/assignments/"+id(assignment)+"/attempts", nil)
	attemptID := id(session["attempt"].(map[string]any))
	studentQuestion := session["questions"].([]any)[0].(map[string]any)
	blanks, _ := studentQuestion["blanks"].([]any)
	if len(blanks) != 1 {
		t.Fatalf("the student's question has %d blanks: %v", len(blanks), studentQuestion)
	}
	blank := blanks[0].(map[string]any)["id"].(string)
	typed := d("hà nội")
	if typed == norm.NFC.String(typed) {
		t.Fatal("the typed answer is not decomposed")
	}
	student.must(http.StatusOK, http.MethodPatch, "/app/attempts/"+attemptID+"/answers", map[string]any{
		"sessionId": session["sessionId"],
		"answers":   map[string]any{id(studentQuestion): map[string]any{"type": "fill_blank", "values": map[string]any{blank: typed}}},
	})
	reloaded := student.must(http.StatusOK, http.MethodGet, "/app/attempts/"+attemptID, nil)
	saved := reloaded["answers"].(map[string]any)[id(studentQuestion)].(map[string]any)["values"].(map[string]any)[blank]
	requireComposed(t, "saved answer", saved, "hà nội")

	student.must(http.StatusOK, http.MethodPost, "/app/attempts/"+attemptID+"/submit", map[string]any{"sessionId": session["sessionId"], "reason": "manual"})
	review := teacher.must(http.StatusOK, http.MethodGet, "/teacher/attempts/"+attemptID, nil)
	if earned := review["attempt"].(map[string]any)["score"].(map[string]any)["earned"]; earned != float64(1) {
		t.Fatalf("a decomposed answer to a composed key scored %v, want the full mark", earned)
	}

	note := teacher.must(http.StatusOK, http.MethodPatch, "/teacher/attempts/"+attemptID+"/note", map[string]any{"note": d("Bài làm tốt")})
	requireComposed(t, "teacher note", note["note"], "Bài làm tốt")
}

func TestAPasswordAndAnEmailAreNeverComposed(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)
	d := decomposedText

	classID := teacher.class("Lớp mật khẩu " + nonce(t))
	studentEmail := d("hoc.sinh.é") + "-" + nonce(t) + "@example.com"
	created := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/students", map[string]any{"email": studentEmail, "fullName": "Học viên", "classIds": []string{classID}})
	if got := created["user"].(map[string]any)["email"]; got != studentEmail {
		t.Fatalf("the email was rewritten: %q, want %q", got, studentEmail)
	}
	student := w.browser()
	student.login(studentEmail, created["temporaryPassword"].(string))

	typed := "mật-khẩu-" + d("é") + "-9"
	if typed == norm.NFC.String(typed) {
		t.Fatal("the password is not decomposed")
	}
	student.must(http.StatusNoContent, http.MethodPost, "/auth/change-password", map[string]any{"currentPassword": created["temporaryPassword"], "newPassword": typed})
	w.browser().login(studentEmail, typed)
	status, _ := w.browser().call(http.MethodPost, "/auth/login", map[string]any{"email": studentEmail, "password": norm.NFC.String(typed)})
	if status != http.StatusUnauthorized {
		t.Fatalf("the composed spelling of the password signed in (status %d): the server rewrote a password", status)
	}
}

func TestNoTextComposingLeavesOverItsLimitReachesAStoredCheckAsAServerError(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)
	atLimit := func(n int) string { return strings.Repeat("क़", n) }

	teacher.must(http.StatusBadRequest, http.MethodPost, "/teacher/tests", map[string]any{"title": atLimit(200)})
	teacher.must(http.StatusBadRequest, http.MethodPost, "/teacher/classes", map[string]any{"name": atLimit(120)})
	classID := teacher.class("Lớp giới hạn " + nonce(t))
	teacher.must(http.StatusBadRequest, http.MethodPatch, "/teacher/classes/"+classID, map[string]any{"name": atLimit(120)})
	teacher.must(http.StatusBadRequest, http.MethodPost, "/teacher/students", map[string]any{
		"email": "gioi-han-" + nonce(t) + "@example.com", "fullName": atLimit(200), "classIds": []string{classID},
	})

	_, versionID := teacher.publishedVersion("Đề giới hạn " + nonce(t))
	test := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/tests", map[string]any{"title": "Đề ghi chú " + nonce(t)})
	question := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/questions", map[string]any{"type": "short_answer", "points": 1, "prompt": "Câu hỏi"})
	test = teacher.must(http.StatusOK, http.MethodPatch, "/teacher/tests/"+id(test), map[string]any{
		"expectedUpdatedAt": test["updatedAt"], "sections": []any{map[string]any{"title": "Phần 1", "questionIds": []string{id(question)}}},
	})
	teacher.must(http.StatusBadRequest, http.MethodPost, "/teacher/tests/"+id(test)+"/publish", map[string]any{"changeNote": atLimit(200)})

	assignment := teacher.assign(versionID, classID)
	teacher.must(http.StatusBadRequest, http.MethodPatch, "/teacher/assignments/"+id(assignment), map[string]any{
		"testVersionId": assignment["testVersionId"], "targets": map[string]any{"classIds": []string{classID}, "studentIds": []string{}},
		"window": assignment["window"], "durationMinutes": assignment["durationMinutes"], "maxAttempts": assignment["maxAttempts"],
		"review": assignment["review"], "integrity": assignment["integrity"], "studentNote": atLimit(500),
	})
}
