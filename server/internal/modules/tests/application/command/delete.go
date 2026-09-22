package command

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/cqrs"
)

type Delete struct{ Request domain.Request }
type DeleteHandler struct{ *support.Service }

func (s DeleteHandler) Handle(ctx context.Context, cmd Delete) (cqrs.Nothing, error) {
	return cqrs.Nothing{}, s.Repo.Delete(ctx, cmd.Request, s.Now())
}
