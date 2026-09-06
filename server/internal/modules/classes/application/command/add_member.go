package command

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/opt"
)

type AddMember struct {
	ClassID   string
	UserID    string
	ActorID   string
	IP        string
	UserAgent string
}

type AddMemberHandler struct {
	*support.Service
}

func (s AddMemberHandler) Handle(ctx context.Context, cmd AddMember) (domain.Member, error) {
	member, err := s.Repo.AddMember(ctx, domain.AddMemberInput{
		ClassID: cmd.ClassID, UserID: cmd.UserID, ActorUserID: cmd.ActorID,
		Now: s.Now(), IP: opt.String(cmd.IP), UserAgent: opt.String(cmd.UserAgent),
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
