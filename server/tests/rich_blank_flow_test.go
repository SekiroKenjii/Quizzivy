//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"quizzivy/internal/shared/content"
	"testing"
)

func richBlankPayload() map[string]any {
	return map[string]any{"type": "fill_blank", "prompt": "[2]\t[1]", "points": 2,
		"promptContent": json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"table","rows":[[{"header":false,"rowSpan":1,"colSpan":1,"content":[{"type":"paragraph","content":[{"type":"gap","id":"gap-b","label":"2"}]}]},{"header":false,"rowSpan":1,"colSpan":1,"content":[{"type":"paragraph","content":[{"type":"gap","id":"gap-a","label":"1"}]}]}]]}]}`),
		"blanks":        []any{map[string]any{"ordinal": 1, "gapId": "gap-a", "acceptedAnswers": []string{"one"}}, map[string]any{"ordinal": 2, "gapId": "gap-b", "acceptedAnswers": []string{"two"}}},
	}
}

func assertGapGraph(t *testing.T, question map[string]any, copied bool) map[string]string {
	t.Helper()
	raw, err := json.Marshal(question["promptContent"])
	if err != nil {
		t.Fatal(err)
	}
	doc, err := content.ParseQuestionPrompt(raw)
	if err != nil || doc.PlainText() != "[2]\t[1]" || question["prompt"] != doc.PlainText() {
		t.Fatalf("invalid copied prompt: %v", question)
	}
	remaining := map[string]bool{}
	for _, id := range doc.GapIDs() {
		remaining[id] = true
	}
	answers := map[string]string{}
	for _, row := range question["blanks"].([]any) {
		blank := row.(map[string]any)
		gap := blank["gapId"].(string)
		if !remaining[gap] || copied && (gap == "gap-a" || gap == "gap-b") {
			t.Fatalf("lost or reused gap binding: %v", blank)
		}
		delete(remaining, gap)
		answer := "one"
		if blank["ordinal"] == float64(2) {
			answer = "two"
		}
		answers[blank["id"].(string)] = answer
		if accepted, ok := blank["acceptedAnswers"].([]any); ok && accepted[0] != answer {
			t.Fatal("answer attached to wrong blank")
		}
	}
	if len(remaining) != 0 || len(answers) != 2 {
		t.Fatal("incomplete blank graph")
	}
	return answers
}

func TestRichBlankLifecycleFreezesAndGradesStableBindings(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)
	payload := richBlankPayload()
	question := teacher.must(http.StatusCreated, http.MethodPost, "/admin/questions", payload)
	assertGapGraph(t, question, false)
	assertGapGraph(t, teacher.must(http.StatusCreated, http.MethodPost, "/admin/questions/"+id(question)+"/duplicate", nil), true)
	test := teacher.must(http.StatusCreated, http.MethodPost, "/admin/tests", map[string]any{"title": "Rich gaps " + nonce(t)})
	teacher.must(http.StatusOK, http.MethodPatch, "/admin/tests/"+id(test), map[string]any{"expectedUpdatedAt": test["updatedAt"], "sections": []any{map[string]any{"title": "Table", "questionIds": []string{id(question)}}}})
	version := teacher.must(http.StatusCreated, http.MethodPost, "/admin/tests/"+id(test)+"/publish", nil)
	preview := teacher.must(http.StatusOK, http.MethodGet, "/admin/tests/"+id(test)+"/preview?version=1", nil)
	assertGapGraph(t, preview["questions"].([]any)[0].(map[string]any), false)
	assertNoAnswerFields(t, preview)
	payload["blanks"].([]any)[0].(map[string]any)["acceptedAnswers"] = []string{"changed"}
	teacher.must(http.StatusOK, http.MethodPatch, "/admin/questions/"+id(question), payload)
	current := teacher.must(http.StatusOK, http.MethodGet, "/admin/tests/"+id(test), nil)
	restored := teacher.must(http.StatusOK, http.MethodPost, "/admin/tests/"+id(test)+"/versions/1/draft", map[string]any{"expectedUpdatedAt": current["updatedAt"]})
	copiedID := restored["sections"].([]any)[0].(map[string]any)["questionIds"].([]any)[0].(string)
	assertGapGraph(t, teacher.must(http.StatusOK, http.MethodGet, "/admin/questions/"+copiedID, nil), true)
	classID := teacher.class("Blank class " + nonce(t))
	created := teacher.must(http.StatusCreated, http.MethodPost, "/admin/students", map[string]any{"email": "gaps-" + nonce(t) + "@example.com", "fullName": "Học viên", "classIds": []string{classID}})
	student := w.browser()
	student.login(created["user"].(map[string]any)["email"].(string), created["temporaryPassword"].(string))
	assignment := teacher.assign(id(version), classID)
	session := student.must(http.StatusOK, http.MethodPost, "/app/assignments/"+id(assignment)+"/attempts", nil)
	studentQuestion := session["questions"].([]any)[0].(map[string]any)
	answers := assertGapGraph(t, studentQuestion, false)
	assertNoAnswerFields(t, session)
	attemptID := id(session["attempt"].(map[string]any))
	student.must(http.StatusOK, http.MethodPatch, "/app/attempts/"+attemptID+"/answers", map[string]any{"sessionId": session["sessionId"], "answers": map[string]any{id(studentQuestion): map[string]any{"type": "fill_blank", "values": answers}}})
	reloaded := student.must(http.StatusOK, http.MethodGet, "/app/attempts/"+attemptID, nil)
	assertGapGraph(t, reloaded["questions"].([]any)[0].(map[string]any), false)
	student.must(http.StatusOK, http.MethodPost, "/app/attempts/"+attemptID+"/submit", map[string]any{"sessionId": session["sessionId"], "reason": "manual"})
	result := student.must(http.StatusOK, http.MethodGet, "/app/attempts/"+attemptID+"/result", nil)
	assertGapGraph(t, result["questions"].([]any)[0].(map[string]any), false)
	assertNoAnswerFields(t, result)
	review := teacher.must(http.StatusOK, http.MethodGet, "/admin/attempts/"+attemptID, nil)
	assertGapGraph(t, review["questions"].([]any)[0].(map[string]any), false)
	if review["attempt"].(map[string]any)["score"].(map[string]any)["earned"] != float64(2) {
		t.Fatalf("wrong score for frozen rich blanks: %v", review["attempt"])
	}
}

func TestRichBlankWritesRejectMissingOrDuplicateBindingsAtomically(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)
	question := teacher.must(http.StatusCreated, http.MethodPost, "/admin/questions", richBlankPayload())
	for _, gap := range []any{nil, "missing", "gap-b"} {
		payload := richBlankPayload()
		payload["blanks"].([]any)[0].(map[string]any)["gapId"] = gap
		payload["points"] = 3
		teacher.must(http.StatusBadRequest, http.MethodPatch, "/admin/questions/"+id(question), payload)
		unchanged := teacher.must(http.StatusOK, http.MethodGet, "/admin/questions/"+id(question), nil)
		assertGapGraph(t, unchanged, false)
		if unchanged["points"] != float64(2) {
			t.Fatal("rejected update partially persisted")
		}
	}
}
