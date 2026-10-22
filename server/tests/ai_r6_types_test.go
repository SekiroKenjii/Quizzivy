//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	aiports "quizzivy/internal/modules/ai/application/ports"
)

type r6Assistant struct {
	mu       sync.Mutex
	requests []aiports.Request
}

func (a *r6Assistant) Complete(_ context.Context, req aiports.Request) (aiports.Response, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.requests = append(a.requests, req)
	answer := answerTo(req)
	switch {
	case strings.Contains(string(req.Schema), `"findings"`):
		answer = json.RawMessage(`{"findings":[` +
			`{"question":1,"severity":"likely_error","issue":"The passage never says.","fix":{"kind":"tfng_answer","description":"Change the answer to Not given.","label":"Change to Not given","option":"C","blank":0,"words":0,"text":""}},` +
			`{"question":2,"severity":"minor","issue":"There is no word limit.","fix":{"kind":"word_limit","description":"Add a word limit.","label":"Add word limit","option":"","blank":0,"words":3,"text":""}}]}`)
	case strings.Contains(string(req.Schema), `"criteria"`):
		answer = json.RawMessage(`{"band":7,"criteria":[{"criterion":1,"band":6,"reason":"Few figures."},{"criterion":2,"band":7,"reason":"Mostly accurate."}],"confidence":"medium","reasons":[],"comment":"Add the key figures."}`)
	case strings.Contains(string(req.Schema), `"questions"`):
		answer = json.RawMessage(`{"questions":[{"type":"tfng","prompt":"Parks are a kind of road.","options":["True","False","Not given"],"answers":["False"],"explanation":""}]}`)
	}
	return aiports.Response{Object: answer, Usage: aiports.Usage{InputTokens: 120, OutputTokens: 40}, RequestID: "req_r6"}, nil
}

func (a *r6Assistant) last() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return textOf(a.requests[len(a.requests)-1])
}

func TestTheAssistantWritesTfngFixesTheNewTypesAndGivesAnEssayABandThatGradingTakes(t *testing.T) {
	provider := &r6Assistant{}
	w := bootAssisted(t, provider)
	email, password := w.createStaff("teacher")
	teacher := w.signedIn(email, password)

	passage := strings.Repeat("Parks give a town room to breathe, to play and to meet. ", 3)
	generated := teacher.must(http.StatusOK, http.MethodPost, "/teacher/ai/questions/generate", map[string]any{
		"source": passage, "types": []string{"tfng"}, "count": 1, "level": "b1", "explanations": false, "language": "en",
	})
	items := generated["items"].([]any)
	if first := items[0].(map[string]any); first["type"] != "tfng" || len(first["options"].([]any)) != 3 || first["answerKey"].([]any)[0] != "False" {
		t.Errorf("generated %v, want one tfng question keyed False", items)
	}
	if status, _, body := teacher.callWith(nil, http.MethodPost, "/teacher/ai/questions/generate", map[string]any{
		"standalone": true, "source": "parks", "types": []string{"tfng"}, "count": 1, "level": "b1", "explanations": false, "language": "en",
	}); status != http.StatusBadRequest || !strings.Contains(asJSON(body), `"types"`) {
		t.Errorf("a standalone tfng run answered %d %v, want a refusal on types", status, body)
	}

	tfng := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/questions", map[string]any{
		"type": "tfng", "prompt": "Car-free zones reduce air pollution.", "points": 1,
		"options": []map[string]any{{"text": "True", "isCorrect": false}, {"text": "False", "isCorrect": true}, {"text": "Not given", "isCorrect": false}},
	})
	short := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/questions", map[string]any{
		"type": "short_answer", "prompt": "Why were early maps inaccurate?", "points": 1, "sampleAnswer": "poor tools",
	})
	essay := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/questions", map[string]any{
		"type": "essay", "prompt": "Summarise the chart.", "points": 9,
		"rubric": []map[string]any{{"label": "Task achievement"}, {"label": "Grammar"}},
	})
	test := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/tests", map[string]any{"title": "Đề R6 có trợ lý " + nonce(t)})
	testID := id(test)
	teacher.must(http.StatusOK, http.MethodPatch, "/teacher/tests/"+testID, map[string]any{
		"expectedUpdatedAt": test["updatedAt"],
		"sections":          []map[string]any{{"title": "Phần 1", "questionIds": []string{id(tfng), id(short), id(essay)}}},
	})

	checked := teacher.must(http.StatusOK, http.MethodPost, "/teacher/ai/tests/"+testID+"/check", map[string]any{"checks": []string{"key", "clarity"}})
	findings := checked["findings"].([]any)
	if len(findings) != 2 {
		t.Fatalf("%d findings, want 2: %v", len(findings), findings)
	}
	notGiven := tfng["options"].([]any)[2].(map[string]any)
	tfngFix := findings[0].(map[string]any)["fix"].(map[string]any)
	tfngPayload, _ := tfngFix["payload"].(map[string]any)
	if tfngFix["kind"] != "tfng_answer" || tfngPayload["optionId"] != notGiven["id"] || tfngPayload["text"] != "Not given" || tfngFix["label"] != "Change to Not given" {
		t.Errorf("the tfng fix reads %v, want the stored Not given option %v as the new key", tfngFix, notGiven["id"])
	}
	limitFix := findings[1].(map[string]any)["fix"].(map[string]any)
	limitPayload, _ := limitFix["payload"].(map[string]any)
	if limitFix["kind"] != "word_limit" || limitPayload["maxWords"] != float64(3) || limitPayload["text"] != "Write NO MORE THAN THREE WORDS." {
		t.Errorf("the word-limit fix reads %v, want a limit of three", limitFix)
	}
	if sent := provider.last(); !strings.Contains(sent, "Q1 [tfng]") || !strings.Contains(sent, "Word limit: none") {
		t.Errorf("the check prompt lacks the tfng question or the missing word limit:\n%s", sent)
	}

	options := []map[string]any{}
	for _, o := range tfng["options"].([]any) {
		option := o.(map[string]any)
		options = append(options, map[string]any{"id": option["id"], "text": option["text"], "isCorrect": option["id"] == tfngPayload["optionId"]})
	}
	teacher.must(http.StatusOK, http.MethodPatch, "/teacher/questions/"+id(tfng), map[string]any{"type": "tfng", "prompt": tfng["prompt"], "points": 1, "options": options})
	teacher.must(http.StatusOK, http.MethodPatch, "/teacher/questions/"+id(short), map[string]any{
		"type": "short_answer", "prompt": short["prompt"], "points": 1, "sampleAnswer": "poor tools", "maxWords": limitPayload["maxWords"],
	})
	if got := teacher.must(http.StatusOK, http.MethodGet, "/teacher/questions/"+id(short), nil); got["maxWords"] != float64(3) {
		t.Errorf("the short answer reads maxWords %v after the fix was written, want 3", got["maxWords"])
	}
	if got := teacher.must(http.StatusOK, http.MethodGet, "/teacher/questions/"+id(tfng), nil); !got["options"].([]any)[2].(map[string]any)["isCorrect"].(bool) {
		t.Errorf("the tfng question reads %v after the fix was written, want Not given correct", got["options"])
	}

	version := id(teacher.must(http.StatusCreated, http.MethodPost, "/teacher/tests/"+testID+"/publish", nil))
	classID := teacher.class("Lớp R6 " + nonce(t))
	studentEmail := "r6-" + nonce(t) + "@example.com"
	created := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/students", map[string]any{"email": studentEmail, "fullName": "Học Viên R6 " + nonce(t), "classIds": []string{classID}})
	student := w.settledIn(studentEmail, created["temporaryPassword"].(string))
	assignment := id(teacher.assign(version, classID))
	session := student.must(http.StatusOK, http.MethodPost, "/app/assignments/"+assignment+"/attempts", nil)
	attempt := id(session["attempt"].(map[string]any))
	var essayID string
	for _, q := range session["questions"].([]any) {
		if question := q.(map[string]any); question["type"] == "essay" {
			essayID = id(question)
		}
	}
	text := "The chart shows a considerable rise in cycling."
	student.must(http.StatusOK, http.MethodPatch, "/app/attempts/"+attempt+"/answers", map[string]any{
		"sessionId": session["sessionId"], "answers": map[string]any{essayID: map[string]any{"type": "text", "value": text}},
	})
	student.must(http.StatusOK, http.MethodPost, "/app/attempts/"+attempt+"/submit", map[string]any{"sessionId": session["sessionId"], "reason": "manual"})

	suggestion := teacher.must(http.StatusOK, http.MethodPost, "/teacher/ai/attempts/"+attempt+"/answers/"+essayID+"/score-suggestion", nil)
	criteria, _ := suggestion["criteria"].([]any)
	reasons, _ := suggestion["reasons"].([]any)
	if suggestion["band"] != 6.5 || suggestion["score"] != 6.5 || len(criteria) != 2 || len(reasons) != 2 || reasons[0] != "Task achievement: Few figures." {
		t.Fatalf("the essay suggestion reads %v, want band 6.5 from two criteria with one reason each", suggestion)
	}
	sent := provider.last()
	if !strings.Contains(sent, text) || !strings.Contains(sent, "1. Task achievement\n2. Grammar") {
		t.Errorf("the score prompt lacks the essay or its criteria:\n%s", sent)
	}
	for _, c := range criteria {
		if strings.Contains(sent, c.(map[string]any)["criterionId"].(string)) {
			t.Error("a criterion id reached the prompt")
		}
	}

	graded := teacher.must(http.StatusOK, http.MethodPost, "/teacher/attempts/"+attempt+"/grade", map[string]any{
		"items": []map[string]any{{"questionId": essayID, "criteria": criteria}},
	})
	if graded == nil {
		t.Fatal("grading with the suggested criteria answered nothing")
	}
	review := teacher.must(http.StatusOK, http.MethodGet, "/teacher/attempts/"+attempt, nil)
	if !strings.Contains(asJSON(review), `"band":6.5`) {
		t.Errorf("the review after grading with the suggestion lacks band 6.5: %s", asJSON(review))
	}
}

func asJSON(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}
