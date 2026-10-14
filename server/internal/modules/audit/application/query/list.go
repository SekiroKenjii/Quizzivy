package query

import (
	"context"
	"strings"

	"quizzivy/internal/modules/audit/application/internal/support"
	"quizzivy/internal/modules/audit/domain"
	"quizzivy/internal/shared/paging"
)

// List reads one page of the log for the viewer: the entries of Category (all
// when empty) that match Query within the range, newest first, and how many
// entries each category holds under the same search and range. Range is Week
// when empty; From and To belong to Custom alone and are YYYY-MM-DD in the
// viewer's zone. A page or limit out of bounds is clamped.
type List struct {
	ViewerID string
	Category domain.Category
	Query    string
	Range    domain.Range
	From     string
	To       string
	Page     int
	Limit    int
}

// ListResult is a page of entries, the counts beside it and where it stands.
type ListResult struct {
	Entries []domain.Entry
	Facets  domain.Facets
	Page    paging.Page
}

type ListHandler struct {
	*support.Service
}

func (s ListHandler) Handle(ctx context.Context, q List) (ListResult, error) {
	if q.Category != "" && !q.Category.Valid() {
		return ListResult{}, domain.ErrUnknownCategory
	}
	_, loc, err := s.LocationOf(ctx, q.ViewerID)
	if err != nil {
		return ListResult{}, err
	}
	window, err := domain.ResolveWindow(q.Range, q.From, q.To, loc, s.Now())
	if err != nil {
		return ListResult{}, err
	}
	search := strings.TrimSpace(q.Query)
	filter := domain.Filter{Category: q.Category, Query: search, Actions: domain.MatchingActions(search), Window: window}

	facets, err := s.Repo.Facets(ctx, filter)
	if err != nil {
		return ListResult{}, err
	}
	number, size, offset := paging.Clamp(q.Page, q.Limit, domain.DefaultLimit, domain.MaxLimit)
	total := facets.Of(q.Category)
	result := ListResult{Entries: []domain.Entry{}, Facets: facets, Page: paging.Page{Number: number, Size: size, Total: total}}
	if offset >= total {
		return result, nil
	}
	records, err := s.Repo.List(ctx, filter, offset, size)
	if err != nil {
		return ListResult{}, err
	}
	result.Entries, err = s.Present(ctx, records)
	if err != nil {
		return ListResult{}, err
	}
	return result, nil
}
