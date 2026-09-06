package command

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/opt"
)

type Create struct {
	Name        string
	Description *string
	SelfJoin    bool
	ActorID     string
	IP          string
	UserAgent   string
}

type CreateHandler struct {
	*support.Service
}

func (s CreateHandler) Handle(ctx context.Context, cmd Create) (domain.Class, error) {
	return s.Repo.Create(ctx, domain.CreateInput{
		Name: cmd.Name, Description: cmd.Description, SelfJoinEnabled: cmd.SelfJoin, ActorUserID: cmd.ActorID,
		Now: s.Now(), IP: opt.String(cmd.IP), UserAgent: opt.String(cmd.UserAgent),
	})
}
