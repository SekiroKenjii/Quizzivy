//go:build e2e

package e2e

import (
	"net/http"
	"slices"
	"testing"

	"github.com/google/uuid"
)

func childIDs(question map[string]any, key string) []string {
	var out []string
	items, _ := question[key].([]any)
	for _, item := range items {
		out = append(out, item.(map[string]any)["id"].(string))
	}
	return out
}

func writeBodyOf(question map[string]any) map[string]any {
	body := map[string]any{"type": question["type"], "prompt": question["prompt"], "points": question["points"]}
	if content := question["promptContent"]; content != nil {
		body["promptContent"] = content
	}
	var options, blanks []any
	for _, raw := range question["options"].([]any) {
		option := raw.(map[string]any)
		write := map[string]any{"id": option["id"], "text": option["text"], "isCorrect": option["isCorrect"]}
		if content := option["content"]; content != nil {
			write["content"] = content
		}
		options = append(options, write)
	}
	for _, raw := range question["blanks"].([]any) {
		blank := raw.(map[string]any)
		blanks = append(blanks, map[string]any{
			"id": blank["id"], "gapId": blank["gapId"], "ordinal": blank["ordinal"],
			"acceptedAnswers": blank["acceptedAnswers"], "caseSensitive": blank["caseSensitive"],
		})
	}
	if options != nil {
		body["options"] = options
	}
	if blanks != nil {
		body["blanks"] = blanks
	}
	return body
}

func TestSavingWhatTheEditorReadKeepsEveryOptionAndBlankId(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)
	path := func(question map[string]any) string { return "/teacher/questions/" + id(question) }

	single := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/questions", map[string]any{
		"type": "single_choice", "prompt": "Chọn " + nonce(t), "points": 1,
		"options": []any{
			map[string]any{"text": "A", "isCorrect": true},
			map[string]any{"text": "B", "isCorrect": false},
			map[string]any{"text": "C", "isCorrect": false},
		},
	})
	options := childIDs(single, "options")
	saved := teacher.must(http.StatusOK, http.MethodPatch, path(single), writeBodyOf(single))
	if got := childIDs(saved, "options"); !slices.Equal(got, options) {
		t.Fatalf("saving the question as it was read changed the option ids: %v, want %v", got, options)
	}

	rotated := writeBodyOf(saved)
	list := rotated["options"].([]any)
	rotated["options"] = []any{list[2], list[0], list[1]}
	saved = teacher.must(http.StatusOK, http.MethodPatch, path(single), rotated)
	if got, want := childIDs(saved, "options"), []string{options[2], options[0], options[1]}; !slices.Equal(got, want) {
		t.Fatalf("option ids after a rotation: %v, want %v", got, want)
	}
	if first := saved["options"].([]any)[0].(map[string]any); first["text"] != "C" || first["ordinal"] != float64(0) {
		t.Fatalf("the option that moved to the top is %v, want C at ordinal 0", first)
	}

	cloze := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/questions", map[string]any{
		"type": "fill_blank", "prompt": "Tôi {{1}} đi học và {{2}} về nhà " + nonce(t), "points": 1,
		"blanks": []any{
			map[string]any{"ordinal": 1, "acceptedAnswers": []string{"đi"}},
			map[string]any{"ordinal": 2, "acceptedAnswers": []string{"về"}, "caseSensitive": true},
		},
	})
	blanks := childIDs(cloze, "blanks")
	saved = teacher.must(http.StatusOK, http.MethodPatch, path(cloze), writeBodyOf(cloze))
	if got := childIDs(saved, "blanks"); !slices.Equal(got, blanks) {
		t.Fatalf("saving the question as it was read changed the blank ids: %v, want %v", got, blanks)
	}

	swapped := writeBodyOf(saved)
	list = swapped["blanks"].([]any)
	first, second := list[0].(map[string]any), list[1].(map[string]any)
	first["ordinal"], second["ordinal"] = 2, 1
	second["acceptedAnswers"] = []string{"về", "ve"}
	saved = teacher.must(http.StatusOK, http.MethodPatch, path(cloze), swapped)
	if got, want := childIDs(saved, "blanks"), []string{blanks[1], blanks[0]}; !slices.Equal(got, want) {
		t.Fatalf("blank ids after swapping the ordinals: %v, want %v", got, want)
	}
	if top := saved["blanks"].([]any)[0].(map[string]any); !slices.Equal(anyStrings(top["acceptedAnswers"]), []string{"ve", "về"}) {
		t.Fatalf("the answers did not follow the blank: %v", top)
	}
	reread := teacher.must(http.StatusOK, http.MethodGet, path(cloze), nil)
	if got := childIDs(reread, "blanks"); !slices.Equal(got, []string{blanks[1], blanks[0]}) {
		t.Fatalf("blank ids read back: %v", got)
	}
}

func anyStrings(value any) []string {
	var out []string
	for _, item := range value.([]any) {
		out = append(out, item.(string))
	}
	return out
}

func TestSavingAGroupAsItWasReadKeepsEveryMembersOptionAndBlankId(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)

	lead := id(teacher.must(http.StatusCreated, http.MethodPost, "/teacher/questions", choice("Câu dẫn "+nonce(t))))
	test := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/tests", map[string]any{"title": "Đề nhóm " + nonce(t)})
	test = teacher.must(http.StatusOK, http.MethodPatch, "/teacher/tests/"+id(test), map[string]any{
		"expectedUpdatedAt": test["updatedAt"], "sections": []map[string]any{{"title": "Phần 1", "questionIds": []string{lead}}},
	})
	section := test["sections"].([]any)[0].(map[string]any)["id"].(string)

	members := []string{uuid.NewString(), uuid.NewString()}
	bundle := groupOf("Nhóm giữ mã", members, []any{})
	questions := bundle["questions"].([]any)
	questions[1].(map[string]any)["input"] = map[string]any{
		"type": "fill_blank", "prompt": "Tôi {{1}} đi học", "points": 1,
		"blanks": []map[string]any{{"ordinal": 1, "acceptedAnswers": []string{"đi"}}},
	}
	group := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/question-groups", map[string]any{
		"bundle": bundle, "ownerSectionId": section, "expectedTestUpdatedAt": test["updatedAt"],
	})
	groupID := group["bundle"].(map[string]any)["group"].(map[string]any)["id"].(string)

	idsOf := func(group map[string]any) (options, blanks []string) {
		for _, raw := range group["bundle"].(map[string]any)["questions"].([]any) {
			input := raw.(map[string]any)["input"].(map[string]any)
			options = append(options, childIDs(input, "options")...)
			blanks = append(blanks, childIDs(input, "blanks")...)
		}
		return options, blanks
	}
	stored := teacher.must(http.StatusOK, http.MethodGet, "/teacher/question-groups/"+groupID, nil)
	options, blanks := idsOf(stored)
	if len(options) != 2 || len(blanks) != 1 {
		t.Fatalf("the stored group holds %d options and %d blanks, want 2 and 1", len(options), len(blanks))
	}

	saved := teacher.must(http.StatusOK, http.MethodPut, "/teacher/question-groups/"+groupID, map[string]any{
		"bundle": stored["bundle"], "expectedRevision": stored["revision"], "expectedTestUpdatedAt": stored["testUpdatedAt"],
	})
	if gotOptions, gotBlanks := idsOf(saved); !slices.Equal(gotOptions, options) || !slices.Equal(gotBlanks, blanks) {
		t.Fatalf("saving the group as it was read changed ids: options %v, want %v; blanks %v, want %v", gotOptions, options, gotBlanks, blanks)
	}
	reread := teacher.must(http.StatusOK, http.MethodGet, "/teacher/question-groups/"+groupID, nil)
	if gotOptions, gotBlanks := idsOf(reread); !slices.Equal(gotOptions, options) || !slices.Equal(gotBlanks, blanks) {
		t.Fatalf("the group read back holds other ids: options %v, want %v; blanks %v, want %v", gotOptions, options, gotBlanks, blanks)
	}
}
