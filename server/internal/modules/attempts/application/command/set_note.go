package command

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/content"
	"quizzivy/internal/shared/cqrs"
)

// SetNote keeps or clears the note on a paper of an assignment Scope reaches.
type SetNote struct {
	AttemptID string
	Note      *string
	Scope     access.Scope
}

type SetNoteHandler struct {
	*support.Review
}

func (r SetNoteHandler) Handle(ctx context.Context, cmd SetNote) (cqrs.Nothing, error) {
	var c content.Composer
	note := c.Optional("note", cmd.Note, domain.MaxTeacherNote)
	if err := c.Err(); err != nil {
		return cqrs.Nothing{}, err
	}
	return cqrs.Nothing{}, r.Repo.SetNote(ctx, cmd.Scope, cmd.AttemptID, note)
}
