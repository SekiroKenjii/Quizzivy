package query

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/access"
)

// AnswersForQuestion lists one question's answers on an assignment Scope
// reaches.
type AnswersForQuestion struct {
	AssignmentID string
	QuestionID   string
	Scope        access.Scope
}

type AnswersForQuestionHandler struct {
	*support.Review
}

func (r AnswersForQuestionHandler) Handle(ctx context.Context, q AnswersForQuestion) (domain.ByQuestion, error) {
	question, err := r.Repo.AnswersForQuestion(ctx, q.Scope, q.AssignmentID, q.QuestionID)
	if err != nil {
		return domain.ByQuestion{}, err
	}
	return r.QuestionContext(ctx, question)
}
