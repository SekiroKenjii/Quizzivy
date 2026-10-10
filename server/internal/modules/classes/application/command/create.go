package command

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/opt"
)

// Create makes a class the actor teaches. A schedule label or room that is
// blank once trimmed is stored as none.
type Create struct {
	Name          string
	Description   *string
	ScheduleLabel *string
	Room          *string
	SelfJoin      bool
	Actor         actor.Actor
}

type CreateHandler struct {
	*support.Service
}

func (s CreateHandler) Handle(ctx context.Context, cmd Create) (domain.Class, error) {
	class, err := s.Repo.Create(ctx, domain.CreateInput{
		Name: cmd.Name, Description: cmd.Description, SelfJoinEnabled: cmd.SelfJoin, ActorUserID: cmd.Actor.ID,
		ScheduleLabel: domain.LabelOf(cmd.ScheduleLabel), Room: domain.LabelOf(cmd.Room),
		Now: s.Now(), IP: opt.String(cmd.Actor.IP), UserAgent: opt.String(cmd.Actor.UserAgent),
	})
	if err != nil {
		return domain.Class{}, err
	}
	return class, s.AttachAverage(ctx, &class)
}
