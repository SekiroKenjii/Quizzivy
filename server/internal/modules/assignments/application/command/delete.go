package command

import (
	"context"
	"quizzivy/internal/modules/assignments/application/internal/support"
	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/shared/cqrs"
	"time"
)

type Delete struct {
	Request domain.Request
	Now     time.Time
}
type DeleteHandler struct{ *support.Service }

func (s DeleteHandler) Handle(ctx context.Context, cmd Delete) (cqrs.Nothing, error) {
	return cqrs.Nothing{}, s.Repo.Delete(ctx, cmd.Request, cmd.Now)
}
