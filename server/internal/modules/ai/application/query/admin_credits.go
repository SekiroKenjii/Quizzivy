package query

import (
	"context"

	"quizzivy/internal/modules/ai/application/internal/support"
	"quizzivy/internal/modules/ai/domain"
)

// AdminCredits is the Admin's view of the current month: the pool, the teachers with their standing, the requests waiting and the rules. The projection is the credits used divided by the days elapsed, times the days in the month, computed here from the clock, and WithinPool says it fits the total.
type AdminCredits struct{}

type AdminCreditsHandler struct {
	*support.Service
}

func (h AdminCreditsHandler) Handle(ctx context.Context, _ AdminCredits) (domain.AdminCredits, error) {
	now := h.Now()
	view, err := h.Repo.Admin(ctx, domain.MonthOf(now, h.Zone))
	if err != nil {
		return domain.AdminCredits{}, err
	}
	view.DaysToGo = domain.DaysToGo(now, h.Zone)
	view.OnPace = domain.OnPace(view.Pool.Used, now, h.Zone)
	view.WithinPool = view.OnPace <= view.Pool.Total()
	return view, nil
}
