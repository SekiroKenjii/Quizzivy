package command

import (
	"context"
	"quizzivy/internal/modules/media/application/internal/support"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/content"
)

// Update renames an asset of the library or sets its default play limit, and
// answers the asset as the library lists it. The name is trimmed. A blank or
// over-long name, a limit out of range or on an image, and an input that
// names no change are refused before anything is written.
type Update struct {
	Input domain.UpdateInput
}

type UpdateHandler struct {
	*support.Service
}

func (s UpdateHandler) Handle(ctx context.Context, cmd Update) (domain.Asset, error) {
	in := cmd.Input
	if in.DisplayName == nil && !in.SetDefaultMaxPlays {
		return domain.Asset{}, domain.ErrNothingToUpdate
	}
	if in.DisplayName != nil {
		name, err := domain.Assets.DisplayName(content.NFC(*in.DisplayName))
		if err != nil {
			return domain.Asset{}, err
		}
		in.DisplayName = &name
	}
	current, err := s.Repo.Find(ctx, access.Scope{UserID: in.ActorID, All: in.All}, in.ID)
	if err != nil {
		return domain.Asset{}, err
	}
	if in.SetDefaultMaxPlays {
		if err := domain.Assets.CheckPlayLimit(current.Kind, in.DefaultMaxPlays); err != nil {
			return domain.Asset{}, err
		}
	}
	if in.Now.IsZero() {
		in.Now = s.Now()
	}
	updated, err := s.Repo.Update(ctx, in)
	if err != nil {
		return domain.Asset{}, err
	}
	assets := []domain.Asset{updated}
	if err := s.Describe(ctx, assets); err != nil {
		return domain.Asset{}, err
	}
	return assets[0], nil
}
