package command

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
)

type RecordPlay struct {
	AttemptID  string
	StudentID  string
	QuestionID string
}

type RecordPlayHandler struct {
	*support.Service
}

func (s RecordPlayHandler) Handle(ctx context.Context, cmd RecordPlay) (domain.Plays, error) {
	return s.Store.RecordPlay(ctx, cmd.AttemptID, cmd.StudentID, cmd.QuestionID, s.Now())
}
