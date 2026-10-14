package query

import (
	"context"
	"time"

	"quizzivy/internal/modules/dashboard/application/internal/support"
	"quizzivy/internal/modules/dashboard/domain"
)

// Usage reads what the centre uses against its plan for Viewer, who is expected to hold system.settings.write. The AI credits are read only for a viewer who holds ai.credits.manage and only while the assistant is configured.
type Usage struct {
	Viewer domain.AdminViewer
}

type UsageHandler struct{ *support.Service }

func (s UsageHandler) Handle(ctx context.Context, q Usage) (domain.Usage, error) {
	credits, err := s.CreditsView(ctx, q.Viewer)
	if err != nil {
		return domain.Usage{}, err
	}
	accounts, err := s.Repo.Accounts(ctx, time.Time{})
	if err != nil {
		return domain.Usage{}, err
	}
	return s.Usage(ctx, accounts, credits)
}
