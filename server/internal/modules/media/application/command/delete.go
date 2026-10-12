package command

import (
	"context"
	"quizzivy/internal/modules/media/application/internal/support"
	"quizzivy/internal/modules/media/domain"
)

// Delete soft-deletes an unreferenced asset and answers the file as it left the
// library, with a URL to download it and the time its object goes.
type Delete struct {
	Input domain.DeleteInput
}

type DeleteHandler struct {
	*support.Service
}

func (s DeleteHandler) Handle(ctx context.Context, cmd Delete) (domain.Retired, error) {
	if cmd.Input.Now.IsZero() {
		cmd.Input.Now = s.Now()
	}
	retired, err := s.Repo.SoftDelete(ctx, cmd.Input)
	if err != nil {
		return domain.Retired{}, err
	}
	return s.SignRetired(ctx, retired)
}
