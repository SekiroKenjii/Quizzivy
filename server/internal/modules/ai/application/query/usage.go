package query

import (
	"context"

	"quizzivy/internal/modules/ai/application/internal/support"
	"quizzivy/internal/modules/ai/domain"
)

// Usage is one teacher's month: what they used, their allowance, what is left of it and of the pool, whether they are paused, whether the centre takes requests for more, their pending request and the five category sums. It creates nothing, so a teacher who has not used AI reads the allowance a first use would give.
type Usage struct {
	UserID string
}

type UsageHandler struct {
	*support.Service
}

func (h UsageHandler) Handle(ctx context.Context, q Usage) (domain.MonthUsage, error) {
	if q.UserID == "" {
		return domain.MonthUsage{}, domain.ErrNotATeacher
	}
	return h.Repo.Usage(ctx, h.Month(), q.UserID)
}
