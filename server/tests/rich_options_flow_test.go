//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
)

func richWord() map[string]any {
	return map[string]any{"format": "semantic_v1", "blocks": []any{map[string]any{
		"type": "paragraph", "content": []any{
			map[string]any{"type": "text", "text": "th", "marks": []any{"underline"}},
			map[string]any{"type": "text", "text": "ink", "marks": []any{}},
		},
	}}}
}

func firstOption(t *testing.T, question map[string]any) map[string]any {
	t.Helper()
	return question["options"].([]any)[0].(map[string]any)
}

func assertRichWord(t *testing.T, question map[string]any) {
	t.Helper()
	option := firstOption(t, question)
	if option["text"] != "think" || !reflect.DeepEqual(option["content"], richWord()) {
		t.Fatalf("option content lost or changed: %v", option)
	}
}

func TestRichOptionsSurviveAuthoringVersionRestoreAndStudentDelivery(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)
	payload := map[string]any{
		"type": "single_choice", "prompt": "Choose the underlined sound " + nonce(t), "points": 1,
		"options": []any{map[string]any{"text": "think", "content": richWord(), "isCorrect": true},
			map[string]any{"text": "other", "isCorrect": false}},
	}
	question := teacher.must(http.StatusCreated, http.MethodPost, "/admin/questions", payload)
	assertRichWord(t, question)
	assertRichWord(t, teacher.must(http.StatusOK, http.MethodGet, "/admin/questions/"+id(question), nil))
	duplicate := teacher.must(http.StatusCreated, http.MethodPost, "/admin/questions/"+id(question)+"/duplicate", nil)
	assertRichWord(t, duplicate)
	if firstOption(t, duplicate)["id"] == firstOption(t, question)["id"] {
		t.Fatal("duplicate reuses an option identity")
	}
	test := teacher.must(http.StatusCreated, http.MethodPost, "/admin/tests", map[string]any{"title": "Rich option flow " + nonce(t)})
	teacher.must(http.StatusOK, http.MethodPatch, "/admin/tests/"+id(test), map[string]any{
		"expectedUpdatedAt": test["updatedAt"],
		"sections":          []any{map[string]any{"title": "Pronunciation", "questionIds": []string{id(question)}}},
	})
	version := teacher.must(http.StatusCreated, http.MethodPost, "/admin/tests/"+id(test)+"/publish", nil)
	options := payload["options"].([]any)
	options[0].(map[string]any)["content"] = nil
	options[0].(map[string]any)["text"] = "changed bank text"
	teacher.must(http.StatusOK, http.MethodPatch, "/admin/questions/"+id(question), payload)
	preview := teacher.must(http.StatusOK, http.MethodGet, "/admin/tests/"+id(test)+"/preview?version=1", nil)
	assertRichWord(t, preview["questions"].([]any)[0].(map[string]any))
	current := teacher.must(http.StatusOK, http.MethodGet, "/admin/tests/"+id(test), nil)
	restored := teacher.must(http.StatusOK, http.MethodPost, "/admin/tests/"+id(test)+"/versions/1/draft", map[string]any{
		"expectedUpdatedAt": current["updatedAt"],
	})
	copiedID := restored["sections"].([]any)[0].(map[string]any)["questionIds"].([]any)[0].(string)
	if copiedID == id(question) {
		t.Fatal("restoration reused the bank question")
	}
	assertRichWord(t, teacher.must(http.StatusOK, http.MethodGet, "/admin/questions/"+copiedID, nil))
	classID := teacher.class("Rich class " + nonce(t))
	created := teacher.must(http.StatusCreated, http.MethodPost, "/admin/students", map[string]any{
		"email": "rich-student-" + nonce(t) + "@example.com", "fullName": "Học viên mẫu", "classIds": []string{classID},
	})
	student := w.browser()
	student.login(created["user"].(map[string]any)["email"].(string), created["temporaryPassword"].(string))
	assignment := teacher.assign(id(version), classID)
	session := student.must(http.StatusOK, http.MethodPost, "/app/assignments/"+id(assignment)+"/attempts", nil)
	studentQuestion := session["questions"].([]any)[0].(map[string]any)
	assertRichWord(t, studentQuestion)
	assertNoAnswerFields(t, studentQuestion)
	attemptID := id(session["attempt"].(map[string]any))
	reloaded := student.must(http.StatusOK, http.MethodGet, "/app/attempts/"+attemptID, nil)
	assertRichWord(t, reloaded["questions"].([]any)[0].(map[string]any))
	student.must(http.StatusOK, http.MethodPost, "/app/attempts/"+attemptID+"/submit", map[string]any{
		"sessionId": session["sessionId"], "reason": "manual",
	})
	result := student.must(http.StatusOK, http.MethodGet, "/app/attempts/"+attemptID+"/result", nil)
	assertRichWord(t, result["questions"].([]any)[0].(map[string]any))
	assertNoAnswerFields(t, result)
	review := teacher.must(http.StatusOK, http.MethodGet, "/admin/attempts/"+attemptID, nil)
	assertRichWord(t, review["questions"].([]any)[0].(map[string]any))
}

func assertNoAnswerFields(t *testing.T, value any) {
	t.Helper()
	switch node := value.(type) {
	case map[string]any:
		for key, child := range node {
			switch key {
			case "isCorrect", "acceptedAnswers", "sampleAnswer", "transcript":
				t.Fatalf("learner payload contains %s", key)
			}
			assertNoAnswerFields(t, child)
		}
	case []any:
		for _, child := range node {
			assertNoAnswerFields(t, child)
		}
	}
}

func TestRichOptionRequestsRejectHiddenKeysAndDuplicateProperties(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)
	for _, raw := range []string{
		`{"format":"semantic_v1","format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"think","marks":[]}]}]}`,
		`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"think","marks":[],"isCorrect":true}]}]}`,
	} {
		teacher.must(http.StatusBadRequest, http.MethodPost, "/admin/questions", map[string]any{
			"type": "single_choice", "prompt": "Invalid rich option", "points": 1,
			"options": []any{map[string]any{"text": "think", "content": json.RawMessage(raw), "isCorrect": true},
				map[string]any{"text": "other", "isCorrect": false}},
		})
	}
}

func TestLegacyEditsCannotSilentlyEraseOptionFormatting(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)
	option := map[string]any{"text": "think", "content": richWord(), "isCorrect": true}
	payload := map[string]any{"type": "single_choice", "prompt": "Original", "points": 1,
		"options": []any{option, map[string]any{"text": "other", "isCorrect": false}}}
	question := teacher.must(http.StatusCreated, http.MethodPost, "/admin/questions", payload)
	option["id"] = firstOption(t, question)["id"]
	delete(option, "content")
	question = teacher.must(http.StatusOK, http.MethodPatch, "/admin/questions/"+id(question), payload)
	assertRichWord(t, question)
	teacher.must(http.StatusBadRequest, http.MethodPatch, "/admin/questions/"+id(question), payload)
	option["id"] = firstOption(t, question)["id"]
	option["text"], payload["prompt"] = "lost marks", "Should roll back"
	teacher.must(http.StatusBadRequest, http.MethodPatch, "/admin/questions/"+id(question), payload)
	unchanged := teacher.must(http.StatusOK, http.MethodGet, "/admin/questions/"+id(question), nil)
	assertRichWord(t, unchanged)
	if unchanged["prompt"] != "Original" {
		t.Fatal("failed update changed the question")
	}
	option["content"] = nil
	cleared := teacher.must(http.StatusOK, http.MethodPatch, "/admin/questions/"+id(question), payload)
	if firstOption(t, cleared)["content"] != nil || firstOption(t, cleared)["text"] != "lost marks" {
		t.Fatal("explicit removal did not produce the requested plain option")
	}
}
