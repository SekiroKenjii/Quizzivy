package query

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/paging"
)

// ListUsers asks for one page of the Users table.
type ListUsers struct {
	Query domain.AdminUserQuery
}

// ListUsersResult is the page, its position and the tabs' counts.
type ListUsersResult struct {
	Items  []domain.AdminUser
	Page   paging.Page
	Facets domain.AdminUserFacets
}

type ListUsersHandler struct {
	*support.Admin
}

// Handle returns the page with each photo's signed link, and the tabs' counts
// under the query's other filters.
func (h ListUsersHandler) Handle(ctx context.Context, q ListUsers) (ListUsersResult, error) {
	items, page, err := h.Repo.List(ctx, q.Query)
	if err != nil {
		return ListUsersResult{}, err
	}
	for i := range items {
		if items[i].AvatarURL, err = h.Service.AvatarLink(ctx, items[i].AvatarKey); err != nil {
			return ListUsersResult{}, err
		}
	}
	facets, err := h.Repo.Facets(ctx, q.Query)
	if err != nil {
		return ListUsersResult{}, err
	}
	return ListUsersResult{Items: items, Page: page, Facets: facets}, nil
}
