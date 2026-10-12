package command

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/content"
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
	var c content.Composer
	title := c.Text("title", cmd.Title, domain.MaxTestTitle)
	description := c.Optional("description", cmd.Description, 0)
	if err := c.Err(); err != nil {
		return domain.Test{}, err
	}
	return s.Repo.Create(ctx, domain.CreateInput{
		Title:       title,
		Description: description,
		ActorID:     cmd.Request.ActorID,
		OwnerID:     cmd.OwnerID,
		Now:         s.Now(),
		IP:          cmd.Request.IP,
		UserAgent:   cmd.Request.UserAgent,
	})
}
