package command

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/domain"
)

type Create struct {
	Request     domain.Request
	Title       string
	Description *string
}

type CreateHandler struct {
	*support.Service
}

func (s CreateHandler) Handle(ctx context.Context, cmd Create) (domain.Test, error) {
	return s.Repo.Create(ctx, domain.CreateInput{
		Title:       cmd.Title,
		Description: cmd.Description,
		ActorID:     cmd.Request.ActorID,
		Now:         s.Now(),
		IP:          cmd.Request.IP,
		UserAgent:   cmd.Request.UserAgent,
	})
}
