package command

import (
	"context"
	"quizzivy/internal/modules/questions/application/internal/support"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/shared/cqrs"
)

type Delete struct {
	Request domain.WriteRequest
}

type DeleteHandler struct {
	*support.Service
}

func (s DeleteHandler) Handle(ctx context.Context, cmd Delete) (cqrs.Nothing, error) {
	return cqrs.Nothing{}, s.Repo.SoftDelete(ctx, domain.WriteInput{
		ID:        cmd.Request.ID,
		ActorID:   cmd.Request.ActorID,
		Now:       s.Now(),
		IP:        cmd.Request.IP,
		UserAgent: cmd.Request.UserAgent,
	})
}
