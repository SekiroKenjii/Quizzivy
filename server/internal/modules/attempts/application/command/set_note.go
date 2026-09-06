package command

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/shared/cqrs"
)

type SetNote struct {
	AttemptID string
	Note      *string
}

type SetNoteHandler struct {
	*support.Review
}

func (r SetNoteHandler) Handle(ctx context.Context, cmd SetNote) (cqrs.Nothing, error) {
	return cqrs.Nothing{}, r.Repo.SetNote(ctx, cmd.AttemptID, cmd.Note)
}
