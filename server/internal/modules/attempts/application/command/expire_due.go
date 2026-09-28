package command

import (
	"context"
	"quizzivy/internal/shared/access"

	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/shared/cqrs"
)

// ExpireDue closes every attempt on the assignment whose time has run out, so a monitor read after it never shows a live row past its deadline.
// ExpireDue closes the overdue live attempts of an assignment Scope reaches.
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
