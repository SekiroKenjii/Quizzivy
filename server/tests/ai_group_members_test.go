//go:build e2e

package e2e

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

const (
	harbourPassage = "The harbour opened in 1902 and handled mostly salt and timber."
	firstMember    = "When did the harbour open?"
	secondMember   = "What did the harbour handle?"
	loneQuestion   = "Which word names a person who teaches?"
)

func harbourMember(id, prompt string) map[string]any {
	return map[string]any{"id": id, "input": map[string]any{
		"type": "single_choice", "prompt": prompt, "points": 1,
		"options": []map[string]any{{"text": "In 1902", "isCorrect": true}, {"text": "In 1920", "isCorrect": false}},
	}}
}

func (w *world) harbourTest(teacher *client, standalone bool) (test string) {
	w.t.Helper()
	created := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/tests", map[string]any{"title": "Đề chỉ có nhóm " + nonce(w.t)})
	questionIDs := []string{}
	if standalone {
		lone := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/questions", map[string]any{"type": "short_answer", "prompt": loneQuestion, "points": 1, "sampleAnswer": "A teacher."})
		questionIDs = append(questionIDs, id(lone))
	}
	updated := teacher.must(http.StatusOK, http.MethodPatch, "/teacher/tests/"+id(created), map[string]any{
		"expectedUpdatedAt": created["updatedAt"], "sections": []map[string]any{{"title": "Phần 1", "questionIds": questionIDs}},
	})
	first, second := uuid.NewString(), uuid.NewString()
	teacher.must(http.StatusCreated, http.MethodPost, "/teacher/question-groups", map[string]any{
		"bundle": map[string]any{
			"group": map[string]any{
				"id": uuid.NewString(), "title": "Bến cảng",
				"members":    []map[string]any{{"questionId": first, "optionOrder": "fixed"}, {"questionId": second, "optionOrder": "shuffle"}},
				"stimuli":    []map[string]any{{"id": uuid.NewString(), "title": "The harbour", "content": textDoc(harbourPassage), "gaps": []any{}}},
				"recordings": []any{},
			},
			"questions": []map[string]any{harbourMember(first, firstMember), harbourMember(second, secondMember)},
		},
		"ownerSectionId":        updated["sections"].([]any)[0].(map[string]any)["id"],
		"expectedTestUpdatedAt": updated["updatedAt"],
	})
	return id(created)
}

func TestTheAssistantChecksATestMadeOnlyOfGroupMembersInsteadOfFindingNothingToRead(t *testing.T) {
	provider := newAssistant()
	w := bootAssisted(t, provider)
	email, password := w.createStaff("teacher")
	teacher := w.signedIn(email, password)
	test := w.harbourTest(teacher, false)

	checked := teacher.must(http.StatusOK, http.MethodPost, "/teacher/ai/tests/"+test+"/check", map[string]any{"checks": []string{"key"}})
	if findings, ok := checked["findings"].([]any); !ok || len(findings) != 1 {
		t.Fatalf("the check answered %v, want the one finding the provider made", checked)
	}
	sent := provider.sent()
	if len(sent) != 1 {
		t.Fatalf("the provider was asked %d times, want once", len(sent))
	}
	read := textOf(sent[0])
	for _, want := range []string{firstMember, secondMember, harbourPassage, "Q1 [single_choice] (uses P1)", "Q2 [single_choice] (uses P1)"} {
		if !strings.Contains(read, want) {
			t.Errorf("the assistant was not shown %q:\n%s", want, read)
		}
	}
	if n := strings.Count(read, harbourPassage); n != 1 {
		t.Errorf("the passage was sent %d times, want once for both members", n)
	}
}

func TestTheAssistantNumbersAStandaloneQuestionAndAGroupsMembersAsTheBuilderDoesInACheckAndAVariant(t *testing.T) {
	provider := newAssistant()
	w := bootAssisted(t, provider)
	email, password := w.createStaff("teacher")
	teacher := w.signedIn(email, password)
	test := w.harbourTest(teacher, true)

	teacher.must(http.StatusOK, http.MethodPost, "/teacher/ai/tests/"+test+"/check", map[string]any{"checks": []string{"key"}})
	varied := teacher.must(http.StatusOK, http.MethodPost, "/teacher/ai/tests/"+test+"/variant", map[string]any{"changes": []string{"reword"}, "difficulty": "same"})
	if changes, ok := varied["changes"].([]any); !ok || len(changes) != 1 {
		t.Fatalf("the variant answered %v, want the one change the provider proposed", varied)
	}
	sent := provider.sent()
	if len(sent) != 2 {
		t.Fatalf("the provider was asked %d times, want a check and a variant", len(sent))
	}
	check, variant := textOf(sent[0]), textOf(sent[1])
	for _, want := range []string{"Q1 [short_answer]", loneQuestion, "Q2 [single_choice] (uses P1)", "Q3 [single_choice] (uses P1)", firstMember, secondMember} {
		if !strings.Contains(check, want) {
			t.Errorf("the check was not shown %q:\n%s", want, check)
		}
	}
	for _, want := range []string{"Q1 [short_answer]", loneQuestion, "Q2 [single_choice] (uses P1)", "Q3 [single_choice] (uses P1)", firstMember, secondMember, harbourPassage} {
		if !strings.Contains(variant, want) {
			t.Errorf("the variant was not shown %q:\n%s", want, variant)
		}
	}
}

func TestAVariantOfATestMadeOnlyOfGroupMembersVariesTheMembers(t *testing.T) {
	provider := newAssistant()
	w := bootAssisted(t, provider)
	email, password := w.createStaff("teacher")
	teacher := w.signedIn(email, password)
	test := w.harbourTest(teacher, false)

	varied := teacher.must(http.StatusOK, http.MethodPost, "/teacher/ai/tests/"+test+"/variant", map[string]any{"changes": []string{"reword"}, "difficulty": "same"})
	changes, ok := varied["changes"].([]any)
	if !ok || len(changes) != 1 {
		t.Fatalf("the variant answered %v, want the reword of the first member", varied)
	}
	if change, _ := changes[0].(map[string]any); change["number"] != float64(1) || change["before"] != firstMember {
		t.Errorf("the change is %v, want question 1, the first member", changes[0])
	}
	if read := textOf(provider.sent()[0]); !strings.Contains(read, harbourPassage) {
		t.Errorf("the variant was not shown the members' passage:\n%s", read)
	}
}
