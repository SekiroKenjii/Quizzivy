package command

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/content"
)

// CreateStudent adds a student who signs in with the temporary password it returns.
type CreateStudent struct {
	Request domain.WriteRequest
	Input   domain.NewStudent
}

type CreateStudentResult struct {
	Student           domain.Student
	TemporaryPassword string
}

type CreateStudentHandler struct {
	*support.Students
}

func (s CreateStudentHandler) Handle(ctx context.Context, cmd CreateStudent) (CreateStudentResult, error) {
	var c content.Composer
	cmd.Input.FullName = c.Text("fullName", cmd.Input.FullName, domain.MaxFullNameLength)
	if err := c.Err(); err != nil {
		return CreateStudentResult{Student: domain.Student{}, TemporaryPassword: ""}, err
	}
	temporary, hash, err := support.TemporaryPassword(ctx)
	if err != nil {
		return CreateStudentResult{Student: domain.Student{}, TemporaryPassword: ""}, err
	}
	cmd.Input.Hash = hash
	cmd.Input.Now = s.Now()
	request, err := s.Issuing(ctx, cmd.Request)
	if err != nil {
		return CreateStudentResult{Student: domain.Student{}, TemporaryPassword: ""}, err
	}
	student, err := s.Repo.Create(ctx, request, cmd.Input)
	if err != nil {
		return CreateStudentResult{Student: domain.Student{}, TemporaryPassword: ""}, err
	}
	student, err = s.WithStats(ctx, request.Scope(), student)
	return CreateStudentResult{Student: student, TemporaryPassword: temporary}, err
}
