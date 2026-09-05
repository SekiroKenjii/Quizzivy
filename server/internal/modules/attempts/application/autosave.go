package application

import (
	"context"
	"quizzivy/internal/modules/attempts/domain"
)

// Save is the service's side: it owns the clock, and nothing else here needs
// deciding.
func (s *Service) Save(ctx context.Context, in domain.SaveInput) (domain.SaveResult, error) {
	return s.store.Save(ctx, in, s.now())
}
