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
	SessionID  string
}

type RecordPlayHandler struct {
	*support.Service
}

func (s RecordPlayHandler) Handle(ctx context.Context, cmd RecordPlay) (domain.Plays, error) {
	return s.Store.RecordPlay(ctx, domain.PlayInput{
		AttemptID: cmd.AttemptID, StudentID: cmd.StudentID, QuestionID: cmd.QuestionID, SessionID: cmd.SessionID,
	}, s.Now())
}
