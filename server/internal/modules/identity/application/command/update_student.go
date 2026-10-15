package command

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/validation"
)

type UpdateStudent struct {
	Request domain.WriteRequest
	Input   domain.StudentPatch
}

type UpdateStudentHandler struct {
	*support.Students
}

func (s UpdateStudentHandler) Handle(ctx context.Context, cmd UpdateStudent) (domain.Student, error) {
	if err := s.MayActOn(ctx, cmd.Request, cmd.Input.ID, cmd.Input.Disabled != nil); err != nil {
		return domain.Student{}, err
	}
	var refused validation.Error
	if cmd.Input.FullName != nil {
		name := cleanName(&refused, *cmd.Input.FullName)
		cmd.Input.FullName = &name
	}
	if cmd.Input.Email != nil {
		checkAddress(&refused, *cmd.Input.Email)
	}
	if invalid := refused.OrNil(); invalid != nil {
		return domain.Student{}, invalid
	}
	cmd.Input.Now = s.Now()
	student, err := s.Repo.Update(ctx, cmd.Request, cmd.Input)
	if err != nil {
		return domain.Student{}, err
	}
	if cmd.Input.Disabled != nil {
		s.Principals.Forget(cmd.Input.ID)
	}
	return s.WithStats(ctx, cmd.Request.Scope(), student)
}
