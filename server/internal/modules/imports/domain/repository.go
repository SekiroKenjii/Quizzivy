package domain

import "context"

// Repository persists reservations before object writes and atomically advances complete source sets.
type Repository interface {
	Create(context.Context, Create, Quotas) (Import, error)
	Get(context.Context, string) (Import, error)
	List(context.Context, Filter) (List, error)
	Reserve(context.Context, Reserve, Quotas) (Source, error)
	Finish(context.Context, Finish) (Receipt, error)
	Source(context.Context, string, string) (Source, error)
}
