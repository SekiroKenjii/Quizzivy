package query

import (
	"context"
	"quizzivy/internal/modules/media/application/internal/support"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/shared/paging"
)

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
	ids := make([]string, len(assets))
	for i := range assets {
		ids[i] = assets[i].ID
	}
	refs, err := s.Repo.ReferencesFor(ctx, ids)
	if err != nil {
		return ListResult{Items: nil, Page: paging.Page{}}, err
	}
	for i := range assets {
		url, err := s.Object.SignedURL(ctx, assets[i].StorageKey, s.TTL)
		if err != nil {
			return ListResult{Items: nil, Page: paging.Page{}}, err
		}
		assets[i].URL = url
		assets[i].UsedIn = refs[assets[i].ID]
		assets[i].UsageCount = len(assets[i].UsedIn)
	}
	return ListResult{Items: assets, Page: page}, nil
}
