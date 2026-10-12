package query

import (
	"context"
	"quizzivy/internal/modules/media/application/internal/support"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/shared/access"
)

// Usage is the bytes the Scope's whole library holds, beside the quota the
// deployment allows one owner.
type Usage struct {
	Scope access.Scope
}

type UsageHandler struct {
	*support.Service
}

func (s UsageHandler) Handle(ctx context.Context, q Usage) (domain.Usage, error) {
	usage, err := s.Repo.Usage(ctx, q.Scope)
	if err != nil {
		return domain.Usage{}, err
	}
	usage.QuotaBytes = s.Quota
	return usage, nil
}
