package application

import (
	"context"
	"quizzivy/internal/modules/attempts/domain"
)

func (s *Service) RecordPlay(ctx context.Context, attemptID, studentID, questionID string) (domain.Plays, error) {
	return s.store.RecordPlay(ctx, attemptID, studentID, questionID, s.now())
}
