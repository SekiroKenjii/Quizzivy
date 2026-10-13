package query

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/access"
)

// WrongAnswers reads, for each question of an assignment Scope reaches, the distinct answer texts that did not earn full points over the papers handed in by the students Scope reaches. It backs the AI insights and no operation of its own, so no permission names it.
type WrongAnswers struct {
	AssignmentID string
	Scope        access.Scope
}

// WrongAnswersHandler closes the attempts that ran out of time before it reads, as the item analysis does, so both read the same papers.
type WrongAnswersHandler struct {
	*support.Service
}

func (h WrongAnswersHandler) Handle(ctx context.Context, q WrongAnswers) (domain.WrongAnswers, error) {
	if err := h.ExpireDue(ctx, q.Scope, q.AssignmentID); err != nil {
		return domain.WrongAnswers{}, err
	}
	return h.Store.WrongAnswers(ctx, q.Scope, q.AssignmentID)
}
