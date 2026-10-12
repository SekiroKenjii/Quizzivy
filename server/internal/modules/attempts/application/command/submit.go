package command

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
)

type Submit struct {
	AttemptID string
	StudentID string
	Reason    domain.Reason
}

type SubmitHandler struct {
	*support.Service
}

func (s SubmitHandler) Handle(ctx context.Context, cmd Submit) (domain.Attempt, error) {
	closed, reached, err := s.Store.Submit(ctx, cmd.AttemptID, cmd.StudentID, cmd.Reason, s.Now())
	if err != nil {
		return closed.Attempt, err
	}
	s.Announcer.Announce(ctx, reached)
	return closed.Attempt, nil
}
