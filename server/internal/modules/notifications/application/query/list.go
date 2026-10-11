package query

import (
	"context"

	"quizzivy/internal/modules/notifications/application/internal/support"
	"quizzivy/internal/modules/notifications/domain"
)

// List reads one page of UserID's own notifications older than Before,
// newest first, sized as domain.ListQuery.Size says. A non-empty Audience
// keeps only that console's kinds; one that is neither the teacher's nor the
// student's is refused.
type List struct {
	UserID   string
	Before   string
	Limit    int
	Audience domain.Audience
}

type ListHandler struct {
	*support.Service
}

func (s ListHandler) Handle(ctx context.Context, q List) (domain.Page, error) {
	switch {
	case q.UserID == "":
		return domain.Page{}, domain.ErrNoRecipient
	case !q.Audience.Valid():
		return domain.Page{}, domain.ErrUnknownAudience
	}
	return s.Repo.List(ctx, domain.ListQuery{UserID: q.UserID, Before: q.Before, Limit: q.Limit, Audience: q.Audience})
}
