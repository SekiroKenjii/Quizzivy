//go:build e2e

package e2e

import (
	"context"
	"net/http"
	"testing"
)

func TestQuestionValidationGuardsHTTPAndPublication(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)
	payload := map[string]any{"type": "true_false", "prompt": "Đọc và chọn", "points": 1,
		"options": []any{map[string]any{"text": "Đúng", "isCorrect": true}, map[string]any{"text": "Sai", "isCorrect": true}}}
	teacher.must(http.StatusBadRequest, http.MethodPost, "/admin/questions", payload)
	payload["options"].([]any)[1].(map[string]any)["isCorrect"] = false
	for _, points := range []float64{0, -1, 1.005, 1000000} {
		payload["points"] = points
		teacher.must(http.StatusBadRequest, http.MethodPost, "/admin/questions", payload)
	}
	for _, points := range []float64{0.01, 0.29, 1.5, 999999.99} {
		payload["points"] = points
		question := teacher.must(http.StatusCreated, http.MethodPost, "/admin/questions", payload)
		if question["points"] != points {
			t.Fatalf("score changed: %v instead of %v", question["points"], points)
		}
	}
	payload["points"] = 1
	question := teacher.must(http.StatusCreated, http.MethodPost, "/admin/questions", payload)
	test := teacher.must(http.StatusCreated, http.MethodPost, "/admin/tests", map[string]any{"title": "Shared validation " + nonce(t)})
	teacher.must(http.StatusOK, http.MethodPatch, "/admin/tests/"+id(test), map[string]any{"expectedUpdatedAt": test["updatedAt"], "sections": []any{map[string]any{"title": "Phần 1", "questionIds": []string{id(question)}}}})
	if _, err := w.pool.Exec(context.Background(), `UPDATE app.question_options SET is_correct = true WHERE question_id = $1`, id(question)); err != nil {
		t.Fatal(err)
	}
	blocked := teacher.must(http.StatusConflict, http.MethodPost, "/admin/tests/"+id(test)+"/publish", nil)
	violations := blocked["violations"].([]any)
	if len(violations) != 1 || violations[0].(map[string]any)["questionId"] != id(question) {
		t.Fatalf("publication did not identify the invalid bank question: %v", violations)
	}
	var count int
	if err := w.pool.QueryRow(context.Background(), `SELECT count(*) FROM app.test_versions WHERE test_id = $1`, id(test)).Scan(&count); err != nil || count != 0 {
		t.Fatalf("invalid publication left a snapshot: %d, %v", count, err)
	}
	teacher.must(http.StatusOK, http.MethodPatch, "/admin/questions/"+id(question), payload)
	teacher.must(http.StatusCreated, http.MethodPost, "/admin/tests/"+id(test)+"/publish", nil)
}
