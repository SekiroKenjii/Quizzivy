package query

import (
	"context"
	"errors"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
)

// Get is §7's rule that a student fetches test content through exactly one
// endpoint. It reads the paper and never changes the session. A reader that
// names the attempt's session, or none, is given it with a fresh beacon token;
// one that names another session is given the paper marked superseded, and
// nothing is written for it.
type Get struct {
	AttemptID   string
	StudentID   string
	HeldSession string
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
	rules, err := s.Store.RulesFor(ctx, attempt.AssignmentID, attempt.StudentID)
	if err != nil {
		return domain.Session{}, err
	}
	if q.HeldSession != "" && q.HeldSession != attempt.SessionID && attempt.Status == domain.InProgress {
		return s.superseded(ctx, attempt, q.HeldSession, rules)
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

func (s GetHandler) superseded(ctx context.Context, attempt domain.AttemptRecord, held string, rules domain.Rules) (domain.Session, error) {
	attempt.SessionID = held
	session, err := s.Session(ctx, attempt, "", rules)
	if err != nil {
		return domain.Session{}, err
	}
	session.Superseded = true
	return session, nil
}
