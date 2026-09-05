package application

import (
	"context"
	"quizzivy/internal/modules/attempts/domain"
)

func (s *Service) Flush(ctx context.Context, in domain.FlushInput) error {
	return s.store.Flush(ctx, in, s.now())
}
