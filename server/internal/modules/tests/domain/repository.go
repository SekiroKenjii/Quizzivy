package domain

import (
	"context"
	"time"

	"quizzivy/internal/shared/paging"
)

// Repository persists tests, their drafts and the versions published from them.
type Repository interface {
	List(ctx context.Context, in ListInput) ([]Test, paging.Page, error)
	Facets(ctx context.Context, in ListInput) (StatusFacets, error)
	Tags(ctx context.Context, in ListInput) ([]string, error)
	Get(ctx context.Context, id string) (Test, error)
	Create(ctx context.Context, in CreateInput) (Test, error)
	Update(ctx context.Context, in UpdateRequest) (Test, error)
	Duplicate(ctx context.Context, in DuplicateInput) (Test, error)
	ListVersions(ctx context.Context, testID string) ([]Version, error)
	Preview(ctx context.Context, testID string, version int) (int, []PreviewQuestion, error)
	Publish(ctx context.Context, req PublishRequest, now time.Time, validate func(DraftContent) error) (PublishedVersion, error)
}
