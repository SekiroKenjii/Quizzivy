package command

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/application/model"
	"quizzivy/internal/modules/tests/domain"
)

// ArchiveGroup changes only an independent bank group's archive state.
type ArchiveGroup struct {
	Mutation model.GroupMutation
	Archived bool
}
type ArchiveGroupHandler struct{ *support.Groups }

func (s ArchiveGroupHandler) Handle(ctx context.Context, cmd ArchiveGroup) (domain.StoredGroup, error) {
	if s.Repo == nil {
		return domain.StoredGroup{}, domain.ErrGroupUnavailable
	}
	group, err := s.Repo.Get(ctx, cmd.Mutation.ID)
	if err != nil {
		return domain.StoredGroup{}, err
	}
	if group.OwnerSectionID != nil {
		return domain.StoredGroup{}, &domain.GroupError{Rule: "group_bank_only"}
	}
	return s.Repo.SetArchived(ctx, cmd.Mutation.At(s.Now()), cmd.Archived)
}
