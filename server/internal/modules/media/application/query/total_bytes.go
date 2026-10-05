package query

import (
	"context"
	"quizzivy/internal/modules/media/application/internal/support"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/shared/access"
)

// TotalBytes sums the library List shows for the same Scope, Kind, Query and
// Unused.
type TotalBytes struct {
	Kind   *domain.Kind
	Query  string
	Unused bool
	Scope  access.Scope
}

type TotalBytesHandler struct {
	*support.Service
}

func (s TotalBytesHandler) Handle(ctx context.Context, q TotalBytes) (int64, error) {
	return s.Repo.TotalBytes(ctx, domain.ListInput{Scope: q.Scope, Kind: q.Kind, Query: q.Query, Unused: q.Unused})
}
