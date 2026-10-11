package command

import (
	"context"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/shared/content"
)

type Create = domain.Create

type CreateHandler struct {
	Repo   domain.Repository
	Quotas domain.Quotas
}

const maxTitle = 200

func (h CreateHandler) Handle(ctx context.Context, in Create) (domain.Import, error) {
	var c content.Composer
	in.Title = c.Text("title", in.Title, maxTitle)
	if err := c.Err(); err != nil {
		return domain.Import{}, err
	}
	return h.Repo.Create(ctx, in, h.Quotas)
}
