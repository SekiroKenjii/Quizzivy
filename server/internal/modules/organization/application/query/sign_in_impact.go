package query

import (
	"context"
	"strings"

	"quizzivy/internal/modules/organization/application/internal/support"
	"quizzivy/internal/modules/organization/domain"
)

// SignInImpact asks what Policy would do to the active accounts, and whether
// ActorID would be left without a way in. Nothing is stored.
type SignInImpact struct {
	ActorID string
	Policy  domain.SignIn
}

type SignInImpactHandler struct {
	*support.Service
}

func (h SignInImpactHandler) Handle(ctx context.Context, q SignInImpact) (domain.Impact, error) {
	policy := q.Policy
	policy.AllowedGoogleDomains = make([]string, len(q.Policy.AllowedGoogleDomains))
	for i, d := range q.Policy.AllowedGoogleDomains {
		policy.AllowedGoogleDomains[i] = strings.ToLower(strings.TrimSpace(d))
	}
	return h.Repo.Impact(ctx, policy, q.ActorID, h.GoogleConfigured)
}
