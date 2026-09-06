package query

import (
	"context"
	"errors"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
)

// Get is §7's rule that a student fetches test content through exactly one
// endpoint. It re-reads the paper without disturbing the session: a reload
// takes the attempt over, a refetch does not.
type Get struct {
	AttemptID string
	StudentID string
}

type GetHandler struct {
	*support.Service
}

func (s GetHandler) Handle(ctx context.Context, q Get) (domain.Session, error) {
	if err := s.Store.ExpireIfDue(ctx, q.AttemptID, s.Now()); err != nil {
		return domain.Session{}, err
	}
	attempt, err := s.Store.ByID(ctx, q.AttemptID, q.StudentID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Session{}, domain.ErrForbidden
		}
		return domain.Session{}, err
	}
	rules, err := s.Store.RulesFor(ctx, attempt.AssignmentID)
	if err != nil {
		return domain.Session{}, err
	}

	beacon, hash, err := s.NewBeacon()
	if err != nil {
		return domain.Session{}, err
	}
	if err := s.Store.Rebeacon(ctx, attempt.ID, hash); err != nil {
		return domain.Session{}, err
	}
	return s.Session(ctx, attempt, beacon, rules)
}
