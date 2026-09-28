package query

import (
	"context"
	"errors"
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
	out := domain.ActiveJoinCode{IssuedCode: stored.IssuedCode, Legacy: stored.Lookup.Scheme == domain.LookupLegacy}
	if out.Legacy || stored.Lookup.KeyID == nil {
		return out, nil
	}
	code, err := s.Keys.Open(stored.ClassID, stored.ID, *stored.Lookup.KeyID, stored.Ciphertext)
	if errors.Is(err, domain.ErrJoinCodeKeyUnavailable) {
		return out, nil
	}
	if err != nil {
		return domain.ActiveJoinCode{}, err
	}
	out.Code = code
	return out, nil
}
