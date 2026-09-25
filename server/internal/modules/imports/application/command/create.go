package command

import (
	"context"
	"quizzivy/internal/modules/imports/domain"
)

type Create = domain.Create

type CreateHandler struct {
	Repo   domain.Repository
	Quotas domain.Quotas
}

func (h CreateHandler) Handle(ctx context.Context, in Create) (domain.Import, error) {
	return h.Repo.Create(ctx, in, h.Quotas)
}
