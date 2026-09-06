package command

import (
	"context"
	"quizzivy/internal/modules/questions/application/internal/support"
)

type AddTags struct {
	IDs  []string
	Tags []string
}

type AddTagsHandler struct {
	*support.Service
}

func (s AddTagsHandler) Handle(ctx context.Context, cmd AddTags) (int, error) {
	return s.Repo.AddTags(ctx, cmd.IDs, cmd.Tags)
}
