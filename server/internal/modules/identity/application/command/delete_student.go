package command

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/cqrs"
)

type DeleteStudent struct {
	ID      string
	Request domain.WriteRequest
}
type DeleteStudentHandler struct{ *support.Students }

func (s DeleteStudentHandler) Handle(ctx context.Context, cmd DeleteStudent) (cqrs.Nothing, error) {
	if err := s.MayActOn(ctx, cmd.Request, cmd.ID, false); err != nil {
		return cqrs.Nothing{}, err
	}
	if err := s.Repo.Delete(ctx, cmd.Request, cmd.ID, s.Now()); err != nil {
		return cqrs.Nothing{}, err
	}
	s.Principals.Forget(cmd.ID)
	return cqrs.Nothing{}, nil
}
