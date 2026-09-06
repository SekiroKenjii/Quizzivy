package command

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/domain"
)

type Duplicate struct {
	Request domain.Request
}

type DuplicateHandler struct {
	*support.Service
}

func (s DuplicateHandler) Handle(ctx context.Context, cmd Duplicate) (domain.Test, error) {
	return s.Repo.Duplicate(ctx, domain.DuplicateInput{
		ID:        cmd.Request.ID,
		ActorID:   cmd.Request.ActorID,
		Now:       s.Now(),
		IP:        cmd.Request.IP,
		UserAgent: cmd.Request.UserAgent,
	})
}
