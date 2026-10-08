package command

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/domain"
)

type Publish struct {
	Request domain.PublishRequest
}

type PublishHandler struct {
	*support.Publisher
}

func (p PublishHandler) Handle(ctx context.Context, cmd Publish) (domain.Version, error) {
	cmd.Request.ChangeNote = domain.Publishing.ChangeNote(cmd.Request.ChangeNote)
	return p.Repo.Publish(ctx, cmd.Request, p.Now(), domain.Publishing.Validate)
}
