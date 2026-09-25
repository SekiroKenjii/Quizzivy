package command

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/actor"
	"time"
)

// CreateGroup creates one independent bank or section graph.
type CreateGroup struct {
	Bundle                domain.GroupBundle
	OwnerSectionID        *string
	ExpectedTestUpdatedAt time.Time
	Actor                 actor.Actor
}
type CreateGroupHandler struct{ *support.Groups }

func (s CreateGroupHandler) Handle(ctx context.Context, cmd CreateGroup) (domain.StoredGroup, error) {
	bundle, err := s.Prepare(ctx, cmd.Bundle)
	if err != nil {
		return domain.StoredGroup{}, err
	}
	return s.Repo.Create(ctx, domain.CreateGroupInput{Bundle: bundle, OwnerSectionID: cmd.OwnerSectionID, ExpectedTestUpdatedAt: cmd.ExpectedTestUpdatedAt, ActorID: cmd.Actor.ID, IP: cmd.Actor.IP, UserAgent: cmd.Actor.UserAgent, Now: s.Now()})
}
