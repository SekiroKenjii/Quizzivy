package domain

import (
	"context"

	"quizzivy/internal/shared/paging"
)

// Repository persists the question bank.
type Repository interface {
	List(ctx context.Context, in ListInput) ([]Question, paging.Page, error)
	Get(ctx context.Context, id string) (Question, error)
	GetIncludingDeleted(ctx context.Context, id string) (Question, error)
	Create(ctx context.Context, in WriteInput) (Question, error)
	Update(ctx context.Context, in WriteInput) (Question, error)
	SoftDelete(ctx context.Context, in WriteInput) error
	AddTags(ctx context.Context, ids []string, tags []string) (int, error)
	Facets(ctx context.Context, in ListInput) (TypeFacets, error)
	Tags(ctx context.Context, in ListInput) ([]string, error)
	Counts(ctx context.Context, in ListInput) (total int, filtered int, err error)
}
