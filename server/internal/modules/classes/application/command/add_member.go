package command

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/opt"
)

type AddMember struct {
	ClassID string
	UserID  string
	Actor   actor.Actor
}

type AddMemberHandler struct {
	*support.Service
}

func (s AddMemberHandler) Handle(ctx context.Context, cmd AddMember) (domain.Member, error) {
	member, err := s.Repo.AddMember(ctx, domain.AddMemberInput{
		ClassID: cmd.ClassID, UserID: cmd.UserID, ActorUserID: cmd.Actor.ID,
		Now: s.Now(), IP: opt.String(cmd.Actor.IP), UserAgent: opt.String(cmd.Actor.UserAgent),
	})
	if err != nil {
		return domain.Member{}, err
	}
	members := []domain.Member{member}
	if err := s.AttachStats(ctx, members); err != nil {
		return domain.Member{}, err
	}
	return members[0], nil
}
