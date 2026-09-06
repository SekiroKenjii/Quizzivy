package query

import (
	"context"
	"errors"
	"quizzivy/internal/modules/media/application/internal/support"
	"quizzivy/internal/modules/media/application/model"
	"quizzivy/internal/modules/media/domain"
)

type MintForStudent struct {
	StudentID string
	AssetID   string
}

type MintForStudentHandler struct {
	*support.Service
}

func (s MintForStudentHandler) Handle(ctx context.Context, q MintForStudent) (model.SignedURLResult, error) {
	ok, err := s.Repo.ReachableByStudent(ctx, q.StudentID, q.AssetID)
	if err != nil {
		return model.SignedURLResult{}, err
	}
	if !ok {
		return model.SignedURLResult{}, domain.ErrForbidden
	}

	asset, err := s.Repo.Get(ctx, q.AssetID)
	if errors.Is(err, domain.ErrNotFound) {
		return model.SignedURLResult{}, domain.ErrForbidden
	}
	if err != nil {
		return model.SignedURLResult{}, err
	}
	return s.Mint(ctx, asset)
}
