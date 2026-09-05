package application

import (
	"context"
	"errors"
	"quizzivy/internal/modules/attempts/domain"
)

// Result is §9's result page, in the order the student saw the paper.
func (s *Service) Result(ctx context.Context, attemptID, studentID string) (domain.Result, error) {
	if err := s.store.ExpireIfDue(ctx, attemptID, s.now()); err != nil {
		return domain.Result{}, err
	}
	a, err := s.store.ByID(ctx, attemptID, studentID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Result{}, domain.ErrForbidden
		}
		return domain.Result{}, err
	}
	switch a.Status {
	case domain.InProgress:
		return domain.Result{}, domain.ErrAttemptInProgress
	case domain.Voided:
		return domain.Result{}, domain.ErrAttemptVoided
	}
	return s.store.LoadResult(ctx, a)
}
