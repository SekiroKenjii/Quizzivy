package command

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/application/model"
	"quizzivy/internal/modules/tests/domain"
)

// MoveQuestion moves a question across a shared-context group's boundary, between two groups of its test, or within one group.
type MoveQuestion struct{ Move model.QuestionMove }
type MoveQuestionHandler struct{ *support.Groups }

func (s MoveQuestionHandler) Handle(ctx context.Context, cmd MoveQuestion) (domain.MoveQuestionResult, error) {
	if s.Repo == nil {
		return domain.MoveQuestionResult{}, domain.ErrGroupUnavailable
	}
	return s.Repo.MoveQuestion(ctx, cmd.Move.At(s.Now()))
}
