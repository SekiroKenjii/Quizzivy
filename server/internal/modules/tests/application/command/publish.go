package command

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/content"
)

type Publish struct {
	Request domain.PublishRequest
}

type PublishHandler struct {
	*support.Publisher
}

func (p PublishHandler) Handle(ctx context.Context, cmd Publish) (domain.Version, error) {
	var c content.Composer
	note := c.Optional("changeNote", cmd.Request.ChangeNote, domain.MaxChangeNote)
	if err := c.Err(); err != nil {
		return domain.Version{}, err
	}
	cmd.Request.ChangeNote = domain.Publishing.ChangeNote(note)
	return p.Repo.Publish(ctx, cmd.Request, p.Now(), domain.Publishing.Validate)
}
