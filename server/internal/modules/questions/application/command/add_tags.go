package command

import (
	"context"
	"quizzivy/internal/modules/questions/application/internal/support"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/content"
)

// AddTags adds Tags to the bank questions among IDs that Scope reaches; any
// other id is skipped exactly as a missing one is.
type AddTags struct {
	IDs   []string
	Tags  []string
	Scope access.Scope
}

type AddTagsHandler struct {
	*support.Service
}

func (s AddTagsHandler) Handle(ctx context.Context, cmd AddTags) (int, error) {
	return s.Repo.AddTags(ctx, cmd.Scope, cmd.IDs, content.NFCAll(cmd.Tags))
}
