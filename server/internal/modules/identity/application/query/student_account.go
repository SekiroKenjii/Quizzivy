package query

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
)

// StudentAccount reads an account's fields and derived role, without
// memberships or figures, whatever its role. It is not scoped: callers name only a user reached
// through a parent they already read under their own scope, such as the sitter
// of an attempt on an assignment they may review, never an id taken from a
// request.
type StudentAccount struct {
	ID string
}

type StudentAccountHandler struct {
	*support.Students
}

func (s StudentAccountHandler) Handle(ctx context.Context, q StudentAccount) (domain.Account, error) {
	return s.Repo.Account(ctx, q.ID)
}
