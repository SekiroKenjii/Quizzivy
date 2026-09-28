package command

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/domain"
)

// Create makes a new empty draft owned by OwnerID when set, else by the
// request's actor. Only a Word-import commit sets OwnerID.
type Create struct {
	Request     domain.Request
	Title       string
	Description *string
	OwnerID     string
}

type CreateHandler struct {
	*support.Service
}

func (s CreateHandler) Handle(ctx context.Context, cmd Create) (domain.Test, error) {
	return s.Repo.Create(ctx, domain.CreateInput{
		Title:       cmd.Title,
		Description: cmd.Description,
		ActorID:     cmd.Request.ActorID,
		OwnerID:     cmd.OwnerID,
		Now:         s.Now(),
		IP:          cmd.Request.IP,
		UserAgent:   cmd.Request.UserAgent,
	})
}
