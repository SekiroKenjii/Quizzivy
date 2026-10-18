package query

import (
	"context"

	"quizzivy/internal/modules/organization/application/internal/support"
	"quizzivy/internal/modules/organization/domain"
)

// RetentionImpact asks what the retention sweep would do today under Policy.
// Nothing is stored.
type RetentionImpact struct {
	Policy domain.Retention
}

type RetentionImpactHandler struct {
	*support.Service
}

func (h RetentionImpactHandler) Handle(ctx context.Context, q RetentionImpact) (domain.RetentionImpact, error) {
	return h.Repo.RetentionImpact(ctx, q.Policy)
}
