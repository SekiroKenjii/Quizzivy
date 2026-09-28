package command

import (
	"context"
	"quizzivy/internal/shared/access"

	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/shared/cqrs"
)

// ExpireDue closes every overdue live attempt on an assignment Scope reaches,
// so a monitor read after it never shows a live row past its deadline.
type ExpireDue struct {
	AssignmentID string
	Scope        access.Scope
}

type ExpireDueHandler struct {
	*support.Service
}

func (s ExpireDueHandler) Handle(ctx context.Context, cmd ExpireDue) (cqrs.Nothing, error) {
	return cqrs.Nothing{}, s.ExpireDue(ctx, cmd.Scope, cmd.AssignmentID)
}
