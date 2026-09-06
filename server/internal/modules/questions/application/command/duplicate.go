package command

import (
	"context"
	"quizzivy/internal/modules/questions/application/internal/support"
	"quizzivy/internal/modules/questions/domain"
)

type Duplicate struct {
	Request domain.WriteRequest
}

type DuplicateHandler struct {
	*support.Service
}

func (s DuplicateHandler) Handle(ctx context.Context, cmd Duplicate) (domain.Question, error) {
	source, err := s.Repo.Get(ctx, cmd.Request.ID)
	if err != nil {
		return domain.Question{}, err
	}
	cmd.Request.ID = ""
	cmd.Request.Input = support.InputOf(source)
	return s.Write(ctx, cmd.Request, false)
}
