package adapters

import (
	"context"
	"errors"
	"fmt"

	accessquery "quizzivy/internal/modules/access/application/query"
	accessdomain "quizzivy/internal/modules/access/domain"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
)

// Principals is the access module's ResolvePrincipal behind httpx's
// PrincipalResolver, for the permission gate and the docs gate: an unknown
// user becomes httpx.ErrUnknownPrincipal, which the gates answer with 401.
type Principals struct {
	Query cqrs.QueryHandler[accessquery.ResolvePrincipal, access.Principal]
}

var _ httpx.PrincipalResolver = Principals{}

// Resolve returns who userID acts as.
func (p Principals) Resolve(ctx context.Context, userID string) (access.Principal, error) {
	principal, err := p.Query.Handle(ctx, accessquery.ResolvePrincipal{UserID: userID})
	if errors.Is(err, accessdomain.ErrUnknownUser) {
		return access.Principal{}, fmt.Errorf("%w: %v", httpx.ErrUnknownPrincipal, err)
	}
	return principal, err
}
