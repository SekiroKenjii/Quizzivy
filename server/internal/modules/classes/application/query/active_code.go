package query

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/access"
)

// ActiveCode reads back the active join code of a class the scope reaches,
// opening its ciphertext. Another teacher's class answers ErrClassNotFound,
// exactly as a missing one does; a class without an active code answers
// ErrNoActiveCode. A legacy code, or one sealed under a key this server no
// longer holds, comes back without its code.
type ActiveCode struct {
	Scope   access.Scope
	ClassID string
}

type ActiveCodeHandler struct {
	*support.Enrolment
}

func (s ActiveCodeHandler) Handle(ctx context.Context, q ActiveCode) (domain.ActiveJoinCode, error) {
	stored, err := s.Repo.ActiveCode(ctx, q.Scope, q.ClassID)
	if err != nil {
		return domain.ActiveJoinCode{}, err
	}
	return s.Read(stored)
}
