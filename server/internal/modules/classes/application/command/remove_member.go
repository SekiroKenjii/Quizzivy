package command

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/cqrs"
	"quizzivy/internal/shared/opt"
)

type RemoveMember struct {
	ClassID string
	UserID  string
	Actor   actor.Actor
}

type RemoveMemberHandler struct {
	*support.Service
}

func (s RemoveMemberHandler) Handle(ctx context.Context, cmd RemoveMember) (cqrs.Nothing, error) {
	if _, err := s.Repo.Get(ctx, cmd.ClassID); err != nil {
		return cqrs.Nothing{}, err
	}
	return cqrs.Nothing{}, s.Repo.RemoveMember(ctx, domain.RemoveMemberInput{
		ClassID: cmd.ClassID, UserID: cmd.UserID, ActorUserID: cmd.Actor.ID,
		Now: s.Now(), IP: opt.String(cmd.Actor.IP), UserAgent: opt.String(cmd.Actor.UserAgent),
	})
}
