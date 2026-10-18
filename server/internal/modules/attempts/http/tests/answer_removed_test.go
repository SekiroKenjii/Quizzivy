package http_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/attempts/domain"
)

func TestGradingARemovedAnswerAnswersAnswerRemovedNamingTheQuestions(t *testing.T) {
	ids := []string{"01935000-0000-7000-8000-000000000001", "01935000-0000-7000-8000-000000000002"}
	for accept, message := range map[string]string{
		"":   "Câu trả lời đã được xoá theo chính sách lưu trữ của trung tâm, nên không thể chấm.",
		"en": "This answer was removed under the centre's retention policy, so it cannot be graded.",
	} {
		response := teacherRefusedIn(t, accept, grading(&domain.AnswerRemovedError{QuestionIDs: ids}))
		var body struct {
			Error struct {
				Code    openapi.ErrorCode `json:"code"`
				Message string            `json:"message"`
				Details struct {
					QuestionIDs []string `json:"questionIds"`
				} `json:"details"`
			} `json:"error"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatalf("%v in %s", err, response.Body.String())
		}
		if response.Code != http.StatusConflict || body.Error.Code != openapi.ANSWERREMOVED || body.Error.Message != message {
			t.Errorf("Accept-Language %q answered %d %s %q", accept, response.Code, body.Error.Code, body.Error.Message)
		}
		if !slices.Equal(body.Error.Details.QuestionIDs, ids) {
			t.Errorf("details name %v, want %v", body.Error.Details.QuestionIDs, ids)
		}
	}
}
