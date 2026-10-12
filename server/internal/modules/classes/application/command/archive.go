package command

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/opt"
)

type Archive struct {
	ClassID  string
	Archived bool
	Actor    actor.Actor
}

type ArchiveHandler struct {
	*support.Service
}

func (s ArchiveHandler) Handle(ctx context.Context, cmd Archive) (domain.Class, error) {
	class, err := s.Repo.Archive(ctx, domain.ArchiveInput{
		ClassID: cmd.ClassID, Archived: cmd.Archived, ActorUserID: cmd.Actor.ID, All: cmd.Actor.Scope.All,
		Now: s.Now(), IP: opt.String(cmd.Actor.IP), UserAgent: opt.String(cmd.Actor.UserAgent),
	})
	if err != nil {
		return domain.Class{}, err
	}
	return class, s.AttachAverage(ctx, &class)
}
