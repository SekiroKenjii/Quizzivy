package command

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/access"
)

// Update edits the own fields of a class Scope reaches. A schedule label or
// room that is blank once trimmed clears the stored value, as null does.
type Update struct {
	ClassID string
	Input   domain.UpdateInput
	Scope   access.Scope
}

type UpdateHandler struct {
	*support.Service
}

func (s UpdateHandler) Handle(ctx context.Context, cmd Update) (domain.Class, error) {
	input, err := cmd.Input.Composed()
	if err != nil {
		return domain.Class{}, err
	}
	input.ScheduleLabel.Value = domain.LabelOf(input.ScheduleLabel.Value)
	input.Room.Value = domain.LabelOf(input.Room.Value)
	class, err := s.Repo.Update(ctx, cmd.Scope, cmd.ClassID, input)
	if err != nil {
		return domain.Class{}, err
	}
	return class, s.AttachAverage(ctx, &class)
}
