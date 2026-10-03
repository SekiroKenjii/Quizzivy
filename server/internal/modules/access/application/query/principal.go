package query

import (
	"context"

	"quizzivy/internal/modules/access/application/internal/support"
	"quizzivy/internal/shared/access"
)

// ResolvePrincipal asks who a user acts as.
type ResolvePrincipal struct {
	UserID string
}

// ResolvePrincipalHandler answers ResolvePrincipal from the principal cache.
type ResolvePrincipalHandler struct {
	*support.Service
}

// Handle returns the user's principal, or domain.ErrUnknownUser.
func (h ResolvePrincipalHandler) Handle(ctx context.Context, q ResolvePrincipal) (access.Principal, error) {
	return h.Resolve(ctx, q.UserID)
}
