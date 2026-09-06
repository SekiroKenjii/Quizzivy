package command

import (
	"context"
	"errors"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
)

// StartOrResume is §9's entry point: one call whether the student is starting
// fresh, reloading, or arriving on a second device.
type StartOrResume struct {
	AssignmentID string
	StudentID    string
}

type StartOrResumeHandler struct {
	*support.Service
}

func (s StartOrResumeHandler) Handle(ctx context.Context, cmd StartOrResume) (domain.Session, error) {
	var err error
	for range 3 {
		var session domain.Session
		session, err = s.StartOrResume(ctx, cmd.AssignmentID, cmd.StudentID)
		if !errors.Is(err, domain.ErrRaceLost) {
			return session, err
		}
	}
	return domain.Session{}, err
}
