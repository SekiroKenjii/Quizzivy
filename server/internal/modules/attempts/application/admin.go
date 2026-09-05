package application

import (
	"context"
	"quizzivy/internal/modules/attempts/domain"
)

func (s *Service) Extend(ctx context.Context, req domain.Request, attemptID string, minutes int, reason string) (domain.Attempt, error) {
	return s.store.Extend(ctx, req, attemptID, minutes, reason, s.now())
}

func (s *Service) Void(ctx context.Context, req domain.Request, attemptID, reason string) (domain.Attempt, error) {
	return s.store.Void(ctx, req, attemptID, reason, s.now())
}

func (s *Service) Reset(ctx context.Context, req domain.Request, attemptID, reason string) (domain.Attempt, error) {
	return s.store.Reset(ctx, req, attemptID, reason, s.now())
}

func (s *Service) Flag(ctx context.Context, req domain.Request, attemptID string, flagged bool, reason string) (domain.Attempt, error) {
	return s.store.Flag(ctx, req, attemptID, flagged, reason, s.now())
}
