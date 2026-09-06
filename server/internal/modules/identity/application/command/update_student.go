package command

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
)

type UpdateStudent struct {
	Request domain.WriteRequest
	Input   domain.StudentPatch
}

type UpdateStudentHandler struct {
	*support.Students
}

func (s UpdateStudentHandler) Handle(ctx context.Context, cmd UpdateStudent) (domain.Student, error) {
	cmd.Input.Now = s.Now()
	student, err := s.Repo.Update(ctx, cmd.Request, cmd.Input)
	if err != nil {
		return domain.Student{}, err
	}
	return s.WithStats(ctx, student)
}
