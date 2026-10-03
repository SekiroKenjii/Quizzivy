package domain

import (
	"context"
	"quizzivy/internal/shared/access"
)

// Repository persists reservations before object writes and atomically
// advances complete source sets. Reads reach the imports a scope's user
// created, or every one under scope.all; Reserve and Finish reach an import
// their Actor.ID created, or any under Actor.Scope.All. Another creator's
// import answers ErrNotFound exactly as a missing one, before any conflict,
// quota or removal can reveal its state.
type Repository interface {
	Create(context.Context, Create, Quotas) (Import, error)
	Get(context.Context, access.Scope, string) (Import, error)
	List(context.Context, Filter) (List, error)
	Reserve(context.Context, Reserve, Quotas) (Source, error)
	Finish(context.Context, Finish) (Receipt, error)
	Source(context.Context, access.Scope, string, string) (Source, error)
}
