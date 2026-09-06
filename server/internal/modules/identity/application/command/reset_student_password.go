package command

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
)

// ResetStudentPassword issues a fresh temporary password and ends every session the student has.
type ResetStudentPassword struct {
	Request domain.WriteRequest
	ID      string
}

type ResetStudentPasswordHandler struct {
	*support.Students
}

func (s ResetStudentPasswordHandler) Handle(ctx context.Context, cmd ResetStudentPassword) (string, error) {
	temporary, hash, err := support.TemporaryPassword(ctx)
	if err != nil {
		return "", err
	}
	if err := s.Repo.ResetPassword(ctx, cmd.Request, cmd.ID, hash, s.Now()); err != nil {
		return "", err
	}
	return temporary, nil
}
