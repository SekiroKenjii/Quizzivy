package query

import (
	"context"
	"quizzivy/internal/modules/media/application/internal/support"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/shared/paging"
)

// List returns a page of the library with a signed URL on every item, since the
// bucket is private and a listing without URLs cannot render a preview (§11.2).
type List struct {
	Input domain.ListInput
}

type ListResult struct {
	Items []domain.Asset
	Page  paging.Page
}

type ListHandler struct {
	*support.Service
}

func (s ListHandler) Handle(ctx context.Context, q List) (ListResult, error) {
	assets, page, err := s.Repo.List(ctx, q.Input)
	if err != nil {
		return ListResult{Items: nil, Page: paging.Page{}}, err
	}
	if err := s.Describe(ctx, assets); err != nil {
		return ListResult{Items: nil, Page: paging.Page{}}, err
	}
	return ListResult{Items: assets, Page: page}, nil
}
