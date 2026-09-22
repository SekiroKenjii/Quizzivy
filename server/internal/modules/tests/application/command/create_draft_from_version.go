package command

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/domain"
)

type CreateDraftFromVersion struct{ Request domain.VersionRequest }
type CreateDraftFromVersionHandler struct{ *support.Service }

func (s CreateDraftFromVersionHandler) Handle(ctx context.Context, cmd CreateDraftFromVersion) (domain.Test, error) {
	return s.Repo.CreateDraftFromVersion(ctx, cmd.Request, s.Now())
}
