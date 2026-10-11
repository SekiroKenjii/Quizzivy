package command

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/content"
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
	var c content.Composer
	name := c.Text("name", cmd.Name, domain.MaxClassName)
	description := c.Optional("description", cmd.Description, 0)
	scheduleLabel := c.Optional("scheduleLabel", cmd.ScheduleLabel, domain.MaxScheduleLabel)
	room := c.Optional("room", cmd.Room, domain.MaxRoom)
	if err := c.Err(); err != nil {
		return domain.Class{}, err
	}
	class, err := s.Repo.Create(ctx, domain.CreateInput{
		Name: name, Description: description, SelfJoinEnabled: cmd.SelfJoin, ActorUserID: cmd.Actor.ID,
		ScheduleLabel: domain.LabelOf(scheduleLabel), Room: domain.LabelOf(room),
		Now: s.Now(), IP: opt.String(cmd.Actor.IP), UserAgent: opt.String(cmd.Actor.UserAgent),
	})
	if err != nil {
		return domain.Class{}, err
	}
	return class, s.AttachAverage(ctx, &class)
}
