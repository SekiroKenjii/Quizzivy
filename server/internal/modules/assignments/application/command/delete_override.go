package command

import (
	"context"
	"quizzivy/internal/modules/assignments/application/internal/support"
	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/shared/cqrs"
	"time"
)

// DeleteOverride takes one student's override off the assignment Request names.
type DeleteOverride struct {
	Request   domain.Request
	StudentID string
	Now       time.Time
}

type DeleteOverrideHandler struct{ *support.Service }

func (s DeleteOverrideHandler) Handle(ctx context.Context, cmd DeleteOverride) (cqrs.Nothing, error) {
	return cqrs.Nothing{}, s.Repo.DeleteOverride(ctx, cmd.Request, cmd.StudentID, cmd.Now)
}
