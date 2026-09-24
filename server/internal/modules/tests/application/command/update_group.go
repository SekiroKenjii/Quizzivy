package command

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/application/model"
	"quizzivy/internal/modules/tests/domain"
)

// UpdateGroup replaces a complete graph under its observed revisions.
type UpdateGroup struct {
	Mutation model.GroupMutation
	Bundle   domain.GroupBundle
}
type UpdateGroupHandler struct{ *support.Groups }

func (s UpdateGroupHandler) Handle(ctx context.Context, cmd UpdateGroup) (domain.StoredGroup, error) {
	bundle, err := s.Prepare(ctx, cmd.Bundle)
	if err != nil {
		return domain.StoredGroup{}, err
	}
	return s.Repo.Update(ctx, domain.UpdateGroupInput{GroupMutation: cmd.Mutation.At(s.Now()), Bundle: bundle})
}
