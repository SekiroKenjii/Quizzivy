package query

import (
	"context"
	"quizzivy/internal/modules/questions/application/internal/support"
	"quizzivy/internal/modules/questions/domain"
)

type Tags struct {
	Input domain.ListInput
}

type TagsHandler struct {
	*support.Service
}

func (s TagsHandler) Handle(ctx context.Context, q Tags) ([]string, error) {
	return s.Repo.Tags(ctx, q.Input)
}
