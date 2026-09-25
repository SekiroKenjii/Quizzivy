package command

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/application/model"
	"quizzivy/internal/modules/tests/domain"
)

// CopyGroup materializes all context independently at a bank or section destination.
type CopyGroup struct {
	Mutation       model.GroupMutation
	OwnerSectionID *string
}
type CopyGroupHandler struct{ *support.Groups }

func (s CopyGroupHandler) Handle(ctx context.Context, cmd CopyGroup) (domain.StoredGroup, error) {
	if s.Repo == nil {
		return domain.StoredGroup{}, domain.ErrGroupUnavailable
	}
	in := cmd.Mutation.At(s.Now())
	return s.Repo.Copy(ctx, domain.CopyGroupInput{SourceID: in.ID, ExpectedSourceRevision: in.ExpectedRevision, OwnerSectionID: cmd.OwnerSectionID, ExpectedTestUpdatedAt: in.ExpectedTestUpdatedAt, ActorID: in.ActorID, IP: in.IP, UserAgent: in.UserAgent, Now: in.Now})
}
