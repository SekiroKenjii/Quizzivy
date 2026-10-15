package query

import (
	"context"

	"quizzivy/internal/modules/audit/application/internal/support"
	"quizzivy/internal/modules/audit/domain"
)

// Recent reads the Limit newest entries of any category, presented as List presents them, without counting the log: a page that quotes the latest few asks for nothing more. A Limit out of bounds is clamped.
type Recent struct {
	Limit int
}

type RecentHandler struct {
	*support.Service
}

func (s RecentHandler) Handle(ctx context.Context, q Recent) ([]domain.Entry, error) {
	limit := min(max(q.Limit, 1), domain.MaxLimit)
	records, err := s.Repo.List(ctx, domain.Filter{}, 0, limit)
	if err != nil {
		return nil, err
	}
	return s.Present(ctx, records)
}
