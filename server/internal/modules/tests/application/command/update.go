package command

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/domain"
)

type Update struct {
	Request domain.Request
	Input   domain.UpdateInput
}

type UpdateHandler struct {
	*support.Service
}

func (s UpdateHandler) Handle(ctx context.Context, cmd Update) (domain.Test, error) {
	if err := cmd.Input.Validate(); err != nil {
		return domain.Test{}, err
	}
	return s.Repo.Update(ctx, domain.UpdateRequest{
		ID:        cmd.Request.ID,
		Input:     cmd.Input,
		ActorID:   cmd.Request.ActorID,
		Now:       s.Now(),
		IP:        cmd.Request.IP,
		UserAgent: cmd.Request.UserAgent,
	})
}
