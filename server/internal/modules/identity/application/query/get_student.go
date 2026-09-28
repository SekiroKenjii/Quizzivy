package query

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/access"
)

// GetStudent reads one student Scope reaches, with the memberships and
// figures Scope reaches; another teacher's student answers ErrStudentNotFound.
type GetStudent struct {
	ID    string
	Scope access.Scope
}

type GetStudentHandler struct {
	*support.Students
}

func (s GetStudentHandler) Handle(ctx context.Context, q GetStudent) (domain.Student, error) {
	student, err := s.Repo.Get(ctx, q.Scope, q.ID)
	if err != nil {
		return domain.Student{}, err
	}
	return s.WithStats(ctx, q.Scope, student)
}
