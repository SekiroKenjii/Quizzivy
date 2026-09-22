package command

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/domain"
)

type SetCurrentVersion struct{ Request domain.VersionRequest }
type SetCurrentVersionHandler struct{ *support.Service }

func (s SetCurrentVersionHandler) Handle(ctx context.Context, cmd SetCurrentVersion) (domain.Test, error) {
	return s.Repo.SetCurrentVersion(ctx, cmd.Request, s.Now())
}
