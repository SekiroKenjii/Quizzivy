package command

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/cqrs"
)

type DeleteVersion struct{ Request domain.VersionRequest }
type DeleteVersionHandler struct{ *support.Service }

func (s DeleteVersionHandler) Handle(ctx context.Context, cmd DeleteVersion) (cqrs.Nothing, error) {
	return cqrs.Nothing{}, s.Repo.DeleteVersion(ctx, cmd.Request, s.Now())
}
