package command

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/cqrs"
)

type Delete struct {
	ClassID string
	Actor   actor.Actor
}
type DeleteHandler struct{ *support.Service }

func (s DeleteHandler) Handle(ctx context.Context, cmd Delete) (cqrs.Nothing, error) {
	return cqrs.Nothing{}, s.Repo.Delete(ctx, cmd.ClassID, cmd.Actor, s.Now())
}
