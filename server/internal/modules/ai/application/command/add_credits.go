package command

import (
	"context"

	"quizzivy/internal/modules/ai/application/internal/support"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/shared/validation"
)

// AddCredits puts Credits into the current month's pool, which is how an Admin approves use beyond the plan: the pool is never overspent without it (D21). The credits do not carry into next month. The audit row names By as the approver.
type AddCredits struct {
	By      domain.Author
	Credits int64
}

type AddCreditsHandler struct {
	*support.Service
}

func (h AddCreditsHandler) Handle(ctx context.Context, cmd AddCredits) (domain.Pool, error) {
	if err := checkAuthor(cmd.By); err != nil {
		return domain.Pool{}, err
	}
	var refused validation.Error
	if cmd.Credits < 1 || cmd.Credits > domain.MaxAddition {
		refused.Add("credits", "outside what may be added at once")
	}
	if err := refused.OrNil(); err != nil {
		return domain.Pool{}, err
	}
	return h.Repo.AddCredits(ctx, domain.AddCreditsInput{By: cmd.By, Month: h.Month(), Credits: cmd.Credits})
}
