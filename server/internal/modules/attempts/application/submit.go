package application

import (
	"context"
	"quizzivy/internal/modules/attempts/domain"
)

func (s *Service) Submit(ctx context.Context, attemptID, studentID string, reason domain.Reason) (domain.Attempt, error) {
	closed, err := s.store.Submit(ctx, attemptID, studentID, reason, s.now())
	return closed.Attempt, err
}
