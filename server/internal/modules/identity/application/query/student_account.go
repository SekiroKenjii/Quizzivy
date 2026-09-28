package query

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/access"
)

// StudentAccount reads a student's account fields, without memberships or
// figures. It is not scoped: callers name only a student reached through a
// parent they already read under their own scope, such as an attempt on an
// assignment they may review, never an id taken from a request.
type StudentAccount struct {
	ID string
}

type StudentAccountHandler struct {
	*support.Students
}

func (s StudentAccountHandler) Handle(ctx context.Context, q StudentAccount) (domain.Student, error) {
	student, err := s.Repo.Get(ctx, access.Scope{All: true}, q.ID)
	student.Classes = nil
	return student, err
}
