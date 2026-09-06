package query

import (
	"context"
	"errors"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
)

// Result is §9's result page, in the order the student saw the paper.
type Result struct {
	AttemptID string
	StudentID string
}

type ResultHandler struct {
	*support.Service
}

func (s ResultHandler) Handle(ctx context.Context, q Result) (domain.Result, error) {
	if err := s.Store.ExpireIfDue(ctx, q.AttemptID, s.Now()); err != nil {
		return domain.Result{}, err
	}
	a, err := s.Store.ByID(ctx, q.AttemptID, q.StudentID)
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
	return s.Store.LoadResult(ctx, a)
}
