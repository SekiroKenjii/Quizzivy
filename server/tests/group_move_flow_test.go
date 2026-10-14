//go:build e2e

package e2e

import (
	"net/http"
	"slices"
	"testing"

	"github.com/google/uuid"
)

func choice(prompt string) map[string]any {
	return map[string]any{
		"type": "single_choice", "prompt": prompt, "points": 1,
		"options": []map[string]any{{"text": "đúng", "isCorrect": true}, {"text": "sai", "isCorrect": false}},
	}
}

func groupOf(title string, members []string, stimuli []any) map[string]any {
	refs, questions := make([]any, len(members)), make([]any, len(members))
	for i, member := range members {
		refs[i] = map[string]any{"questionId": member, "optionOrder": "shuffle"}
		questions[i] = map[string]any{"id": member, "input": choice("Câu " + member[:8])}
	}
	return map[string]any{
		"group":     map[string]any{"id": uuid.NewString(), "title": title, "members": refs, "stimuli": stimuli, "recordings": []any{}},
		"questions": questions,
	}
}

func (c *client) unitsOf(test map[string]any) []string {
	c.w.t.Helper()
	var out []string
	for _, unit := range test["sections"].([]any)[0].(map[string]any)["units"].([]any) {
		u := unit.(map[string]any)
		out = append(out, u["kind"].(string)+":"+u["id"].(string))
	}
	return out
}

func memberIDsOf(group map[string]any) []string {
	var out []string
	for _, member := range group["bundle"].(map[string]any)["group"].(map[string]any)["members"].([]any) {
		out = append(out, member.(map[string]any)["questionId"].(string))
	}
	return out
}

func errorOf(body map[string]any) (code, rule string) {
	failure := body["error"].(map[string]any)
	if details, ok := failure["details"].(map[string]any); ok {
		rule, _ = details["rule"].(string)
	}
	return failure["code"].(string), rule
}

func TestATeacherMovesQuestionsAcrossAGroupsBoundary(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)

	lead := id(teacher.must(http.StatusCreated, http.MethodPost, "/teacher/questions", choice("Câu dẫn")))
	bank := id(teacher.must(http.StatusCreated, http.MethodPost, "/teacher/questions", choice("Câu ngân hàng")))
	test := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/tests", map[string]any{"title": "Đề chuyển câu " + nonce(t)})
	test = teacher.must(http.StatusOK, http.MethodPatch, "/teacher/tests/"+id(test), map[string]any{
		"expectedUpdatedAt": test["updatedAt"], "sections": []map[string]any{{"title": "Phần 1", "questionIds": []string{lead, bank}}},
	})
	testID, section := id(test), test["sections"].([]any)[0].(map[string]any)["id"].(string)
	read := func() map[string]any {
		return teacher.must(http.StatusOK, http.MethodGet, "/teacher/tests/"+testID, nil)
	}
	createGroup := func(bundle map[string]any) map[string]any {
		return teacher.must(http.StatusCreated, http.MethodPost, "/teacher/question-groups", map[string]any{
			"bundle": bundle, "ownerSectionId": section, "expectedTestUpdatedAt": read()["updatedAt"],
		})
	}
	free := []string{uuid.NewString(), uuid.NewString()}
	group := createGroup(groupOf("Nhóm tự do", free, []any{}))
	groupID := group["bundle"].(map[string]any)["group"].(map[string]any)["id"].(string)
	move := func(question string, groupID *string, index int, revisions ...map[string]any) (int, map[string]any) {
		var target any
		if groupID != nil {
			target = *groupID
		}
		return teacher.call(http.MethodPost, "/teacher/tests/"+testID+"/questions/move", map[string]any{
			"expectedUpdatedAt": read()["updatedAt"], "questionId": question,
			"to":                     map[string]any{"sectionId": section, "groupId": target, "index": index},
			"expectedGroupRevisions": revisions,
		})
	}
	revision := func(group string, n int) map[string]any { return map[string]any{"groupId": group, "revision": n} }

	status, copied := move(bank, &groupID, 1, revision(groupID, 1))
	if status != http.StatusOK || copied["copied"] != true || copied["questionId"] == bank {
		t.Fatalf("copying a bank question into a group answered %d %v", status, copied)
	}
	members := memberIDsOf(copied["groups"].([]any)[0].(map[string]any))
	if !slices.Equal(members, []string{free[0], copied["questionId"].(string), free[1]}) || copied["groups"].([]any)[0].(map[string]any)["revision"].(float64) != 2 {
		t.Fatalf("the copy is not in place or the revision did not move: %v", copied["groups"])
	}
	if got := teacher.unitsOf(copied["test"].(map[string]any)); !slices.Equal(got, []string{"question:" + lead, "group:" + groupID}) {
		t.Fatalf("the saved outline is %v", got)
	}
	teacher.must(http.StatusOK, http.MethodGet, "/teacher/questions/"+bank, nil)

	if status, stale := teacher.call(http.MethodPost, "/teacher/tests/"+testID+"/questions/move", map[string]any{
		"expectedUpdatedAt": test["updatedAt"], "questionId": free[0],
		"to":                     map[string]any{"sectionId": section, "groupId": nil, "index": 0},
		"expectedGroupRevisions": []map[string]any{revision(groupID, 2)},
	}); status != http.StatusConflict {
		t.Fatalf("a replay at the old test revision answered %d %v", status, stale)
	} else if code, _ := errorOf(stale); code != "STALE_WRITE" {
		t.Fatalf("a stale move answered %s", code)
	}

	status, out := move(free[0], nil, 0, revision(groupID, 2))
	if status != http.StatusOK || out["copied"] != false || out["questionId"] != free[0] {
		t.Fatalf("moving a member out answered %d %v", status, out)
	}
	if got := teacher.unitsOf(out["test"].(map[string]any)); !slices.Equal(got, []string{"question:" + free[0], "question:" + lead, "group:" + groupID}) {
		t.Fatalf("the member did not land at the drop position: %v", got)
	}
	teacher.must(http.StatusOK, http.MethodGet, "/teacher/questions/"+free[0], nil)

	status, refused := move(lead, nil, 0, revision(groupID, 3))
	if code, rule := errorOf(refused); status != http.StatusBadRequest || code != "VALIDATION_FAILED" || rule != "move_outline_only" {
		t.Fatalf("a move between two standalone places answered %d %v", status, refused)
	}
	status, refused = move(free[1], &groupID, 0, revision(groupID, 3), revision(uuid.NewString(), 1))
	if code, _ := errorOf(refused); status != http.StatusNotFound || code != "NOT_FOUND" {
		t.Fatalf("a revision for a missing group answered %d %v", status, refused)
	}

	bound := []string{uuid.NewString()}
	material := []any{map[string]any{
		"id": uuid.NewString(), "title": "Bài đọc", "gaps": []any{map[string]any{"kind": "question", "gapId": "choice", "questionId": bound[0]}},
		"content": map[string]any{"format": "semantic_v1", "blocks": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "gap", "id": "choice", "label": "1"}}}}},
	}}
	boundGroup := createGroup(groupOf("Nhóm có chỗ trống", bound, material))
	boundID := boundGroup["bundle"].(map[string]any)["group"].(map[string]any)["id"].(string)
	status, refused = move(bound[0], nil, 0, revision(boundID, 1))
	code, rule := errorOf(refused)
	details := refused["error"].(map[string]any)["details"].(map[string]any)
	if status != http.StatusUnprocessableEntity || code != "VALIDATION_FAILED" || rule != "group_gap_bound" || details["gapId"] != "choice" || details["questionId"] != bound[0] || details["stimulusId"] == "" {
		t.Fatalf("a gap-bound member answered %d %v", status, refused)
	}
	if got := memberIDsOf(teacher.must(http.StatusOK, http.MethodGet, "/teacher/question-groups/"+boundID, nil)); !slices.Equal(got, bound) {
		t.Fatalf("a refused move changed the group: %v", got)
	}
}
