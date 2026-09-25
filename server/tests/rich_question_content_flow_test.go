//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
)

const prosePrompt = "Đọc kỹ\n\nMục\tGiá trị"

func proseDocument() map[string]any {
	cell := func(text string) map[string]any {
		return map[string]any{"header": true, "rowSpan": float64(1), "colSpan": float64(1), "content": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": text, "marks": []any{}}}}}}
	}
	return map[string]any{"format": "semantic_v1", "blocks": []any{
		map[string]any{"type": "heading", "level": float64(2), "content": []any{map[string]any{"type": "text", "text": "Đọc kỹ", "marks": []any{"underline"}}}},
		map[string]any{"type": "table", "rows": []any{[]any{cell("Mục"), cell("Giá trị")}}},
	}}
}

func prosePayload() map[string]any {
	return map[string]any{"type": "single_choice", "prompt": prosePrompt, "promptContent": proseDocument(), "explanation": "think", "explanationContent": richWord(), "points": 1,
		"options": []any{map[string]any{"text": "yes", "isCorrect": true}, map[string]any{"text": "no", "isCorrect": false}}}
}

func assertProse(t *testing.T, question map[string]any, explanation bool) {
	t.Helper()
	if question["prompt"] != prosePrompt || !reflect.DeepEqual(question["promptContent"], proseDocument()) {
		t.Fatalf("lost prompt: %v", question)
	}
	if explanation {
		if question["explanation"] != "think" || !reflect.DeepEqual(question["explanationContent"], richWord()) {
			t.Fatalf("lost explanation: %v", question)
		}
	} else {
		for _, field := range []string{"explanation", "explanationContent"} {
			if _, exists := question[field]; exists {
				t.Fatalf("unreleased %s present", field)
			}
		}
	}
}

func TestQuestionProseSurvivesSnapshotRestoreAndPolicyGatedDelivery(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)
	payload := prosePayload()
	question := teacher.must(http.StatusCreated, http.MethodPost, "/admin/questions", payload)
	assertProse(t, question, true)
	assertProse(t, teacher.must(http.StatusCreated, http.MethodPost, "/admin/questions/"+id(question)+"/duplicate", nil), true)
	test := teacher.must(http.StatusCreated, http.MethodPost, "/admin/tests", map[string]any{"title": "Prose flow " + nonce(t)})
	teacher.must(http.StatusOK, http.MethodPatch, "/admin/tests/"+id(test), map[string]any{"expectedUpdatedAt": test["updatedAt"], "sections": []any{map[string]any{"title": "Reading", "questionIds": []string{id(question)}}}})
	version := teacher.must(http.StatusCreated, http.MethodPost, "/admin/tests/"+id(test)+"/publish", nil)
	payload["prompt"], payload["promptContent"] = "Edited bank", nil
	payload["explanation"], payload["explanationContent"] = "Edited explanation", nil
	teacher.must(http.StatusOK, http.MethodPatch, "/admin/questions/"+id(question), payload)
	preview := teacher.must(http.StatusOK, http.MethodGet, "/admin/tests/"+id(test)+"/preview?version=1", nil)
	assertProse(t, preview["questions"].([]any)[0].(map[string]any), false)
	current := teacher.must(http.StatusOK, http.MethodGet, "/admin/tests/"+id(test), nil)
	restored := teacher.must(http.StatusOK, http.MethodPost, "/admin/tests/"+id(test)+"/versions/1/draft", map[string]any{"expectedUpdatedAt": current["updatedAt"]})
	copiedID := restored["sections"].([]any)[0].(map[string]any)["questionIds"].([]any)[0].(string)
	if copiedID == id(question) {
		t.Fatal("restore reused source identity")
	}
	assertProse(t, teacher.must(http.StatusOK, http.MethodGet, "/admin/questions/"+copiedID, nil), true)
	classID := teacher.class("Prose class " + nonce(t))
	created := teacher.must(http.StatusCreated, http.MethodPost, "/admin/students", map[string]any{"email": "prose-" + nonce(t) + "@example.com", "fullName": "Học viên mẫu", "classIds": []string{classID}})
	student := w.browser()
	student.login(created["user"].(map[string]any)["email"].(string), created["temporaryPassword"].(string))
	assignment := teacher.assign(id(version), classID)
	session := student.must(http.StatusOK, http.MethodPost, "/app/assignments/"+id(assignment)+"/attempts", nil)
	assertProse(t, session["questions"].([]any)[0].(map[string]any), false)
	assertNoAnswerFields(t, session)
	attemptID := id(session["attempt"].(map[string]any))
	reload := student.must(http.StatusOK, http.MethodGet, "/app/attempts/"+attemptID, nil)
	assertProse(t, reload["questions"].([]any)[0].(map[string]any), false)
	student.must(http.StatusOK, http.MethodPost, "/app/attempts/"+attemptID+"/submit", map[string]any{"sessionId": session["sessionId"], "reason": "manual"})
	result := student.must(http.StatusOK, http.MethodGet, "/app/attempts/"+attemptID+"/result", nil)
	assertProse(t, result["questions"].([]any)[0].(map[string]any), false)
	update := map[string]any{}
	for _, key := range []string{"testVersionId", "targets", "window", "durationMinutes", "maxAttempts", "review", "integrity"} {
		update[key] = assignment[key]
	}
	update["targets"] = map[string]any{"classIds": []string{classID}, "studentIds": []string{}}
	update["review"] = map[string]any{"showScore": true, "showCorrectAnswers": false, "showExplanations": true}
	teacher.must(http.StatusOK, http.MethodPatch, "/admin/assignments/"+id(assignment), update)
	result = student.must(http.StatusOK, http.MethodGet, "/app/attempts/"+attemptID+"/result", nil)
	assertProse(t, result["questions"].([]any)[0].(map[string]any), true)
	assertNoAnswerFields(t, result)
	review := teacher.must(http.StatusOK, http.MethodGet, "/admin/attempts/"+attemptID, nil)
	assertProse(t, review["questions"].([]any)[0].(map[string]any), true)
}

func TestLegacyQuestionWritesPreserveProseOrFailAtomically(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)
	payload := prosePayload()
	question := teacher.must(http.StatusCreated, http.MethodPost, "/admin/questions", payload)
	delete(payload, "promptContent")
	delete(payload, "explanationContent")
	payload["points"] = 2
	assertProse(t, teacher.must(http.StatusOK, http.MethodPatch, "/admin/questions/"+id(question), payload), true)
	for _, field := range []string{"prompt", "explanation"} {
		original := payload[field]
		payload[field] = "Changed by old client"
		payload["points"] = 3
		teacher.must(http.StatusBadRequest, http.MethodPatch, "/admin/questions/"+id(question), payload)
		unchanged := teacher.must(http.StatusOK, http.MethodGet, "/admin/questions/"+id(question), nil)
		assertProse(t, unchanged, true)
		if unchanged["points"] != float64(2) {
			t.Fatal("rejected write changed other fields")
		}
		payload[field] = original
	}
	payload["promptContent"], payload["explanationContent"] = nil, nil
	cleared := teacher.must(http.StatusOK, http.MethodPatch, "/admin/questions/"+id(question), payload)
	if cleared["promptContent"] != nil || cleared["explanationContent"] != nil {
		t.Fatal("explicit null did not clear prose")
	}
}

func TestQuestionProseRequestsRejectHiddenKeysAndDuplicateProperties(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)
	for _, field := range []string{"promptContent", "explanationContent"} {
		for _, raw := range []string{
			`{"format":"semantic_v1","format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"think","marks":[]}]}]}`,
			`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[],"acceptedAnswers":["hidden"]}]}`,
			`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"gap","id":"1","label":"1"}]}]}`,
		} {
			payload := prosePayload()
			if field == "promptContent" {
				payload["prompt"] = "think"
			}
			payload[field] = json.RawMessage(raw)
			teacher.must(http.StatusBadRequest, http.MethodPost, "/admin/questions", payload)
		}
	}
}
