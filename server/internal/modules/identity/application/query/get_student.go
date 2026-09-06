package query

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
)

type GetStudent struct {
	ID string
}

type GetStudentHandler struct {
	*support.Students
}

func (s GetStudentHandler) Handle(ctx context.Context, q GetStudent) (domain.Student, error) {
	student, err := s.Repo.Get(ctx, q.ID)
	if err != nil {
		return domain.Student{}, err
	}
	return s.WithStats(ctx, student)
}
