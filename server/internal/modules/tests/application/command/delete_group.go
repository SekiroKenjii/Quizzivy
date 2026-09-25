package command

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/application/model"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/cqrs"
)

// DeleteGroup removes an archived bank graph or a revision-guarded section graph, never a published copy.
type DeleteGroup struct{ Mutation model.GroupMutation }
type DeleteGroupHandler struct{ *support.Groups }

func (s DeleteGroupHandler) Handle(ctx context.Context, cmd DeleteGroup) (cqrs.Nothing, error) {
	if s.Repo == nil {
		return cqrs.Nothing{}, domain.ErrGroupUnavailable
	}
	group, err := s.Repo.Get(ctx, cmd.Mutation.ID)
	if err != nil {
		return cqrs.Nothing{}, err
	}
	if group.OwnerSectionID != nil {
		return cqrs.Nothing{}, s.Repo.RemoveFromSection(ctx, cmd.Mutation.At(s.Now()))
	}
	return cqrs.Nothing{}, s.Repo.Delete(ctx, cmd.Mutation.At(s.Now()))
}
