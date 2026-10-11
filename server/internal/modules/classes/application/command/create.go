package command

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/content"
	"quizzivy/internal/shared/opt"
)

type Create struct {
	Name        string
	Description *string
	SelfJoin    bool
	Actor       actor.Actor
}

type CreateHandler struct {
	*support.Service
}

func (s CreateHandler) Handle(ctx context.Context, cmd Create) (domain.Class, error) {
	var c content.Composer
	name := c.Text("name", cmd.Name, domain.MaxClassName)
	description := c.Optional("description", cmd.Description, 0)
	if err := c.Err(); err != nil {
		return domain.Class{}, err
	}
	return s.Repo.Create(ctx, domain.CreateInput{
		Name: name, Description: description, SelfJoinEnabled: cmd.SelfJoin, ActorUserID: cmd.Actor.ID,
		Now: s.Now(), IP: opt.String(cmd.Actor.IP), UserAgent: opt.String(cmd.Actor.UserAgent),
	})
}
