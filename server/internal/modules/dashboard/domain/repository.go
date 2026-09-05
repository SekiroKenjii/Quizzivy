package domain

import (
	"context"
	"quizzivy/internal/shared/paging"
)

// Repository reads the dashboard's aggregates.
type Repository interface {
	Summary(ctx context.Context) (Summary, error)
	List(ctx context.Context, q ListQuery) ([]Recent, paging.Page, error)
}
