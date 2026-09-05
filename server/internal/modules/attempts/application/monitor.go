package application

import (
	"context"
	"quizzivy/internal/modules/attempts/domain"
)

func (s *Service) Monitor(ctx context.Context, assignmentID string) (domain.Monitor, error) {
	if err := s.expireDue(ctx, assignmentID); err != nil {
		return domain.Monitor{}, err
	}
	return s.store.Monitor(ctx, assignmentID, s.now())
}

// expireDue closes every attempt on the assignment whose time has run out, so
// the monitor never shows "in progress" beside a deadline in the past.
func (s *Service) expireDue(ctx context.Context, assignmentID string) error {
	ids, err := s.store.DueAttempts(ctx, assignmentID, s.now())
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.store.ExpireIfDue(ctx, id, s.now()); err != nil {
			return err
		}
	}
	return nil
}
