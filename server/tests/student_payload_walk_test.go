//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func theKeysOfEachNoticeKind() map[string][]string {
	return map[string][]string{
		"attempt.submitted":   {"title", "count", "toGrade"},
		"attempt.flagged":     {"studentName", "title", "focusLost"},
		"assignment.closing":  {"title", "notSubmitted"},
		"class.joined":        {"studentName", "className"},
		"join_codes.rotated":  {"count", "classNames"},
		"assignment.opened":   {"title", "closesAt"},
		"assignment.due_soon": {"title", "closesAt"},
		"assignment.extended": {"title", "closesAt"},
		"result.ready":        {"title"},
	}
}

func TestNoNoticeOfAnyKindAnswersAKeyOutsideItsKindOrAGradingKey(t *testing.T) {
	w := boot(t)
	email, password := w.createStaff("teacher")
	user := w.signedIn(email, password)
	userID := user.me(t)

	const strays = `"isCorrect":true,"sampleAnswer":"đáp án","acceptedAnswers":["a"],"transcript":"lời","score":9,"email":"x@example.com","reason":"lý do riêng","note":"ghi chú","studentId":"` + "00000000-0000-4000-8000-000000000000" + `"`
	valid := map[string]string{
		"attempt.submitted":   `"title":"Đề","count":2,"toGrade":1`,
		"attempt.flagged":     `"studentName":"Nam","title":"Đề","focusLost":3`,
		"assignment.closing":  `"title":"Đề","notSubmitted":4`,
		"class.joined":        `"studentName":"An","className":"Lớp"`,
		"join_codes.rotated":  `"count":2,"classNames":["Lớp"]`,
		"assignment.opened":   `"title":"Đề","closesAt":"2026-10-12T10:00:00Z"`,
		"assignment.due_soon": `"title":"Đề","closesAt":"2026-10-12T10:00:00Z"`,
		"assignment.extended": `"title":"Đề","closesAt":"2026-10-12T10:00:00Z"`,
		"result.ready":        `"title":"Đề"`,
	}
	allowed := theKeysOfEachNoticeKind()
	if len(valid) != len(allowed) {
		t.Fatalf("the fixtures name %d kinds and the table of keys %d", len(valid), len(allowed))
	}
	for kind, params := range valid {
		if _, err := w.pool.Exec(context.Background(), `
			INSERT INTO app.notifications (user_id, kind, params, target, dedupe_key)
			VALUES ($1::uuid, $2, $3::jsonb, $4::jsonb, $5)`,
			userID, kind, "{"+params+","+strays+"}",
			fmt.Sprintf(`{"route":"result","assignmentId":%q,"attemptId":%q,"isCorrect":true,"reason":"riêng"}`, uuid.NewString(), uuid.NewString()),
			"walk:"+kind); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
	}

	for audience, wantKinds := range map[string]int{"": 9, "?audience=teacher": 5, "?audience=student": 4} {
		page := user.must(http.StatusOK, http.MethodGet, "/me/notifications"+audience, nil)
		items := objects(page["items"])
		if len(items) != wantKinds {
			t.Fatalf("the list %q holds %d notices, want %d: one of every kind it shows", audience, len(items), wantKinds)
		}
		for _, item := range items {
			kind, _ := item["kind"].(string)
			label := "the " + kind + " notice"
			requireNoGradingKeys(t, label, item)
			params, _ := item["params"].(map[string]any)
			for key := range params {
				if !slices.Contains(allowed[kind], key) {
					t.Errorf("%s answers the key %q in its params, which its kind does not name (%v)", label, key, allowed[kind])
				}
			}
			for _, key := range allowed[kind] {
				if _, present := params[key]; !present {
					t.Errorf("%s lost the key %q it was stored with", label, key)
				}
			}
			target, _ := item["target"].(map[string]any)
			for key := range target {
				if !slices.Contains([]string{"route", "assignmentId", "attemptId"}, key) {
					t.Errorf("%s answers the key %q in its target", label, key)
				}
			}
			for key := range item {
				if !slices.Contains([]string{"id", "kind", "params", "target", "createdAt", "readAt"}, key) {
					t.Errorf("%s answers the key %q beside its params", label, key)
				}
			}
		}
	}
}

func TestEveryNoticeTheProductMakesIsWalkedForAGradingKeyOnBothBells(t *testing.T) {
	w := boot(t)
	teacherEmail, teacherPassword := w.createStaff("teacher")
	teacher := w.signedIn(teacherEmail, teacherPassword)
	classID := teacher.class("Lớp đi bộ " + nonce(t))
	code := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/classes/"+classID+"/join-code", map[string]any{})["code"].(string)
	_, versionID := teacher.publishedVersion("Đề đi bộ " + nonce(t))
	now := time.Now()
	open := windowed(teacher, versionID, classID, now.Add(-time.Minute), now.Add(2*time.Hour))
	windowed(teacher, versionID, classID, now.Add(-2*time.Hour), now.Add(30*time.Minute))

	created := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/students", map[string]any{
		"email": "di-bo-" + nonce(t) + "@example.com", "fullName": "Nguyễn Văn An", "classIds": []string{},
	})
	student := w.browser()
	student.login(created["user"].(map[string]any)["email"].(string), created["temporaryPassword"].(string))
	student.must(http.StatusOK, http.MethodPost, "/app/classes/join", map[string]any{"joinCode": code})
	student.sit(id(open))
	teacher.must(http.StatusOK, http.MethodPost, "/teacher/assignments/"+id(open)+"/extend", map[string]any{"minutes": 30, "notify": true})

	seen := map[string]bool{}
	for who, bell := range map[string]*client{"the student's": student, "the teacher's": teacher} {
		for _, n := range bell.inbox(t) {
			kind, _ := n["kind"].(string)
			seen[kind] = true
			requireNoGradingKeys(t, who+" "+kind+" notice", n)
			keys := map[string]bool{}
			keysOf(n, keys)
			if keys["score"] {
				t.Errorf("%s %s notice carries a score", who, kind)
			}
		}
	}
	for _, kind := range []string{"assignment.opened", "assignment.extended", "class.joined", "assignment.closing", "attempt.submitted"} {
		if !seen[kind] {
			t.Errorf("no %s notice was produced, so the walk of the real producers did not reach it (saw %v)", kind, seen)
		}
	}
}

func TestTheStudentSurfaceCarriesNoGradingKeyAndTheAltTextTheTeacherWrote(t *testing.T) {
	w := bootWithStorage(t)
	email, password := w.createStaff("teacher")
	teacher := w.signedIn(email, password)
	tag := nonce(t)
	const alt = "Sơ đồ lớp học có bốn dãy bàn"

	picture := teacher.send(http.MethodPost, "/teacher/media", new(filePayload(t, "so-do.png", tinyPNG(t))))
	if picture.status != http.StatusCreated {
		t.Fatalf("the picture: %d %s", picture.status, picture.body)
	}
	assetID := picture.json["id"].(string)
	question := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/questions", map[string]any{
		"type": "single_choice", "prompt": "Lớp có mấy dãy bàn?", "points": 1, "mediaAssetId": assetID, "mediaAlt": alt,
		"options": []map[string]any{{"text": "ba", "isCorrect": false}, {"text": "bốn", "isCorrect": true}},
	})
	if kept := teacher.must(http.StatusOK, http.MethodGet, "/teacher/questions/"+id(question), nil); kept["mediaAlt"] != alt {
		t.Fatalf("the teacher's question holds the alt text %v, want %q", kept["mediaAlt"], alt)
	}
	keys := map[string]bool{}
	keysOf(teacher.must(http.StatusOK, http.MethodGet, "/teacher/questions/"+id(question), nil), keys)
	if !keys["isCorrect"] {
		t.Fatal("the teacher's own question carries no isCorrect, so the walk below could not tell a leak from a clean payload")
	}

	test := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/tests", map[string]any{"title": "Đề có hình " + tag})
	teacher.must(http.StatusOK, http.MethodPatch, "/teacher/tests/"+id(test), map[string]any{
		"expectedUpdatedAt": test["updatedAt"],
		"sections":          []map[string]any{{"title": "Phần 1", "questionIds": []string{id(question)}}},
	})
	version := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/tests/"+id(test)+"/publish", nil)
	classID := teacher.class("Lớp có hình " + tag)
	now := time.Now()
	assignment := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/assignments", map[string]any{
		"testVersionId":   id(version),
		"targets":         map[string]any{"classIds": []string{classID}, "studentIds": []string{}},
		"window":          map[string]any{"opensAt": rfc3339(now.Add(-time.Minute)), "closesAt": rfc3339(now.Add(2 * time.Hour))},
		"durationMinutes": 30, "maxAttempts": 1,
		"review":      map[string]any{"showScore": true, "showCorrectAnswers": true, "showExplanations": true},
		"integrity":   map[string]any{"requireFullscreen": false, "blockCopyPaste": true, "maxFocusLoss": 0, "onLimitExceeded": "flag", "minAwayMs": 3000},
		"studentNote": "Mang theo bút và thước.",
	})
	student := w.enrolledStudent(teacher, classID)

	walk := func(label, path string) map[string]any {
		t.Helper()
		body := student.must(http.StatusOK, http.MethodGet, path, nil)
		requireNoGradingKeys(t, label, body)
		return body
	}
	walk("the class list", "/app/classes")
	walk("the assignment list", "/app/assignments")
	intro := walk("the assignment's intro", "/app/assignments/"+id(assignment))
	if intro["studentNote"] != "Mang theo bút và thước." {
		t.Errorf("the intro carries the note %v", intro["studentNote"])
	}

	session := student.must(http.StatusOK, http.MethodPost, "/app/assignments/"+id(assignment)+"/attempts", nil)
	requireNoGradingKeys(t, "the attempt as it starts", session)
	attemptID := id(session["attempt"].(map[string]any))
	altOf := func(label string, body map[string]any) {
		t.Helper()
		questions := objects(body["questions"])
		if len(questions) != 1 || questions[0]["mediaAlt"] != alt {
			t.Errorf("%s carries the alt text %v, want %q on its one question", label, questions, alt)
		}
	}
	altOf("the attempt as it starts", session)
	walk("the picture's address", "/app/media/"+assetID+"/url")
	altOf("the attempt as it reloads", walk("the attempt as it reloads", "/app/attempts/"+attemptID))

	submitted := student.must(http.StatusOK, http.MethodPost, "/app/attempts/"+attemptID+"/submit",
		map[string]any{"sessionId": session["sessionId"], "reason": "manual"})
	requireNoGradingKeys(t, "the submitted attempt", submitted)
	result := walk("the result", "/app/attempts/"+attemptID+"/result")
	altOf("the result", result)
	if !strings.Contains(fmt.Sprint(result["review"]), "true") {
		t.Errorf("the result's review policy reads %v, so the walk did not see the answers the teacher allowed", result["review"])
	}
}
