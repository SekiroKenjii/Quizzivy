package command

import (
	"context"
	"quizzivy/internal/modules/assignments/application/internal/support"
	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/shared/content"
)

// SetOverrides gives the students in Input an override, or changes the one
// they have, on the assignment Request names. When Input.Notify is set and it
// names a new close, those students whom the override gave time past the
// assignment's own close are told of theirs once it has committed.
type SetOverrides struct {
	Request domain.Request
	Input   domain.OverrideInput
}

type SetOverridesHandler struct {
	*support.Service
}

func (s SetOverridesHandler) Handle(ctx context.Context, cmd SetOverrides) ([]domain.StudentOverride, error) {
	var c content.Composer
	cmd.Input.Reason = c.Text("reason", cmd.Input.Reason, domain.MaxOverrideReason)
	if err := c.Err(); err != nil {
		return nil, err
	}
	written, err := s.Repo.SetOverrides(ctx, cmd.Request, cmd.Input)
	if err != nil {
		return written, err
	}
	if cmd.Input.Notify && (cmd.Input.ExtendBy != nil || cmd.Input.ClosesAt != nil) {
		s.TellGranted(ctx, cmd.Request.ID, cmd.Input.StudentIDs)
	}
	return written, nil
}
