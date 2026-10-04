package query

import (
	"context"

	"quizzivy/internal/modules/notifications/application/internal/support"
	"quizzivy/internal/modules/notifications/domain"
)

// List reads one page of UserID's own notifications older than Before,
// newest first, sized as domain.ListQuery.Size says.
type List struct {
	UserID string
	Before string
	Limit  int
}

type ListHandler struct {
	*support.Service
}

func (s ListHandler) Handle(ctx context.Context, q List) (domain.Page, error) {
	if q.UserID == "" {
		return domain.Page{}, domain.ErrNoRecipient
	}
	return s.Repo.List(ctx, domain.ListQuery{UserID: q.UserID, Before: q.Before, Limit: q.Limit})
}
