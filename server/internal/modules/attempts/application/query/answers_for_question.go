package query

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
)

type AnswersForQuestion struct {
	AssignmentID string
	QuestionID   string
}

type AnswersForQuestionHandler struct {
	*support.Review
}

func (r AnswersForQuestionHandler) Handle(ctx context.Context, q AnswersForQuestion) (domain.ByQuestion, error) {
	return r.Repo.AnswersForQuestion(ctx, q.AssignmentID, q.QuestionID)
}
