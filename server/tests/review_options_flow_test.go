//go:build e2e

package e2e

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

var gradingKeys = []string{"isCorrect", "sampleAnswer", "acceptedAnswers", "transcript"}

func keysOf(value any, into map[string]bool) {
	switch v := value.(type) {
	case map[string]any:
		for key, inner := range v {
			into[key] = true
			keysOf(inner, into)
		}
	case []any:
		for _, inner := range v {
			keysOf(inner, into)
		}
	}
}

func requireNoGradingKeys(t *testing.T, label string, body map[string]any) {
	t.Helper()
	keys := map[string]bool{}
	keysOf(body, keys)
	for _, key := range gradingKeys {
		if keys[key] {
			t.Errorf("%s carries %q", label, key)
		}
	}
}

func (w *world) enrolledStudent(teacher *client, classID string) *client {
	w.t.Helper()
	created := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/students", map[string]any{
		"email": "student-" + nonce(w.t) + "@example.com", "fullName": "Học viên " + nonce(w.t), "classIds": []string{classID},
	})
	student := w.browser()
	student.login(created["user"].(map[string]any)["email"].(string), created["temporaryPassword"].(string))
	return student
}

func (c *client) sit(assignmentID string) string {
	c.w.t.Helper()
	session := c.must(http.StatusOK, http.MethodPost, "/app/assignments/"+assignmentID+"/attempts", nil)
	attempt := id(session["attempt"].(map[string]any))
	c.must(http.StatusOK, http.MethodPost, "/app/attempts/"+attempt+"/submit",
		map[string]any{"sessionId": session["sessionId"], "reason": "manual"})
	return attempt
}

func TestTheReviewOptionsTheNoteAndTheLockTravelThroughTheAPI(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)

	classID := teacher.class("Lớp công bố " + nonce(t))
	_, versionID := teacher.publishedVersion("Đề công bố " + nonce(t))
	students := []*client{
		w.enrolledStudent(teacher, classID), w.enrolledStudent(teacher, classID), w.enrolledStudent(teacher, classID),
	}

	now := time.Now()
	input := func(mutate func(map[string]any)) map[string]any {
		in := map[string]any{
			"testVersionId":   versionID,
			"targets":         map[string]any{"classIds": []string{classID}, "studentIds": []string{}},
			"window":          map[string]any{"opensAt": rfc3339(now.Add(-time.Minute)), "closesAt": rfc3339(now.Add(2 * time.Hour))},
			"durationMinutes": 30,
			"maxAttempts":     1,
			"review": map[string]any{
				"showScore": true, "showCorrectAnswers": true, "showExplanations": true,
				"release": "after_close", "showClassAverage": true,
			},
			"integrity": map[string]any{
				"requireFullscreen": false, "blockCopyPaste": true, "maxFocusLoss": 0, "onLimitExceeded": "flag", "minAwayMs": 3000,
			},
			"studentNote": "  Mang theo bút và máy tính cầm tay.  ",
		}
		if mutate != nil {
			mutate(in)
		}
		return in
	}

	for name, mutate := range map[string]func(map[string]any){
		"a 501-character note": func(in map[string]any) { in["studentNote"] = strings.Repeat("a", 501) },
		"a hundred attempts":   func(in map[string]any) { in["maxAttempts"] = 100 },
		"an unknown release": func(in map[string]any) {
			in["review"].(map[string]any)["release"] = "never"
		},
	} {
		if status, body := teacher.call(http.MethodPost, "/teacher/assignments", input(mutate)); status != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400: %v", name, status, body)
		}
	}

	assignment := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/assignments", input(nil))
	assignmentID := id(assignment)
	review := assignment["review"].(map[string]any)
	if assignment["studentNote"] != "Mang theo bút và máy tính cầm tay." || review["release"] != "after_close" || review["showClassAverage"] != true {
		t.Fatalf("the teacher's read of what was written: %v", assignment)
	}

	keeps := input(nil)
	delete(keeps, "studentNote")
	delete(keeps["review"].(map[string]any), "release")
	delete(keeps["review"].(map[string]any), "showClassAverage")
	kept := teacher.must(http.StatusOK, http.MethodPatch, "/teacher/assignments/"+assignmentID, keeps)
	if kept["studentNote"] != "Mang theo bút và máy tính cầm tay." || kept["review"].(map[string]any)["release"] != "after_close" || kept["review"].(map[string]any)["showClassAverage"] != true {
		t.Errorf("an update that named none of the three changed them: %v", kept)
	}

	for name, mutate := range map[string]func(map[string]any){
		"the duration": func(in map[string]any) { in["durationMinutes"] = 45 },
		"the attempts": func(in map[string]any) { in["maxAttempts"] = 2 },
	} {
		status, body := teacher.call(http.MethodPatch, "/teacher/assignments/"+assignmentID, input(mutate))
		if code, _ := body["error"].(map[string]any)["code"].(string); status != http.StatusConflict || code != "ASSIGNMENT_LOCKED" {
			t.Errorf("changing %s of an open assignment: status %d code %q, want 409 ASSIGNMENT_LOCKED", name, status, code)
		}
	}
	later := input(func(in map[string]any) {
		in["window"] = map[string]any{"opensAt": rfc3339(now.Add(-time.Minute)), "closesAt": rfc3339(now.Add(3 * time.Hour))}
	})
	teacher.must(http.StatusOK, http.MethodPatch, "/teacher/assignments/"+assignmentID, later)

	intro := students[0].must(http.StatusOK, http.MethodGet, "/app/assignments/"+assignmentID, nil)
	if intro["studentNote"] != "Mang theo bút và máy tính cầm tay." {
		t.Errorf("the student's intro: note %v", intro["studentNote"])
	}
	if r := intro["review"].(map[string]any); r["release"] != "after_close" || r["showClassAverage"] != true {
		t.Errorf("the student's intro: review %v", r)
	}
	requireNoGradingKeys(t, "the intro", intro)

	attempts := make([]string, len(students))
	for i, student := range students {
		attempts[i] = student.sit(assignmentID)
	}
	for i, share := range []float64{8, 6, 4} {
		if _, err := w.pool.Exec(context.Background(), `
			UPDATE app.attempts
			   SET status = 'graded', graded_at = coalesce(graded_at, now()), score_earned = $2, score_total = 10
			 WHERE id = $1::uuid`, attempts[i], share); err != nil {
			t.Fatal(err)
		}
	}

	held := students[0].must(http.StatusOK, http.MethodGet, "/app/attempts/"+attempts[0]+"/result", nil)
	if score, present := held["attempt"].(map[string]any)["score"]; present {
		t.Errorf("a withheld result carries a score: %v", score)
	}
	if _, present := held["classAverage"]; present {
		t.Errorf("a withheld result carries a class average: %v", held["classAverage"])
	}
	if held["releasesAt"] == nil || held["review"].(map[string]any)["showScore"] != false || held["review"].(map[string]any)["release"] != "after_close" {
		t.Errorf("a withheld result does not say it is withheld: %v", held["review"])
	}
	for _, q := range held["questions"].([]any) {
		for _, key := range []string{"earned", "correctOptions", "correctAnswers", "explanation"} {
			if _, present := q.(map[string]any)[key]; present {
				t.Errorf("a withheld result's question carries %q", key)
			}
		}
	}
	requireNoGradingKeys(t, "the withheld result", held)

	closing := input(func(in map[string]any) {
		in["window"] = map[string]any{"opensAt": rfc3339(now.Add(-time.Minute)), "closesAt": rfc3339(now.Add(3 * time.Hour))}
		in["closeNow"] = true
	})
	teacher.must(http.StatusOK, http.MethodPatch, "/teacher/assignments/"+assignmentID, closing)

	released := students[0].must(http.StatusOK, http.MethodGet, "/app/attempts/"+attempts[0]+"/result", nil)
	if _, present := released["releasesAt"]; present {
		t.Errorf("a released result still names a release time: %v", released["releasesAt"])
	}
	if released["attempt"].(map[string]any)["score"] == nil || released["review"].(map[string]any)["showScore"] != true {
		t.Errorf("a released result carries no score: %v", released)
	}
	if released["classAverage"] != 60.0 {
		t.Errorf("class average: %v, want 60 (80, 60 and 40 percent)", released["classAverage"])
	}
	requireNoGradingKeys(t, "the released result", released)

	if _, err := w.pool.Exec(context.Background(), `
		UPDATE app.attempts SET status = 'voided', void_reason = 'reset' WHERE id = $1::uuid`, attempts[2]); err != nil {
		t.Fatal(err)
	}
	fewer := students[0].must(http.StatusOK, http.MethodGet, "/app/attempts/"+attempts[0]+"/result", nil)
	if _, present := fewer["classAverage"]; present {
		t.Errorf("two graded students still give a class average: %v", fewer["classAverage"])
	}

	cleared := input(func(in map[string]any) {
		in["window"] = map[string]any{"opensAt": rfc3339(now.Add(-time.Minute)), "closesAt": rfc3339(now.Add(3 * time.Hour))}
		in["closeNow"] = true
		in["studentNote"] = nil
	})
	after := teacher.must(http.StatusOK, http.MethodPatch, "/teacher/assignments/"+assignmentID, cleared)
	if after["studentNote"] != nil {
		t.Errorf("naming a null note kept %v", after["studentNote"])
	}
	if note, present := students[1].must(http.StatusOK, http.MethodGet, "/app/assignments/"+assignmentID, nil)["studentNote"]; present && note != nil {
		t.Errorf("a cleared note is still shown: %v", note)
	}
}
