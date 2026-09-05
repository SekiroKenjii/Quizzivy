package http

import (
	"context"

	mediadomain "quizzivy/internal/modules/media/domain"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/paging"
)

// Service is the slice of the tests application this transport needs.
type Service interface {
	List(ctx context.Context, in domain.ListInput) ([]domain.Test, paging.Page, error)
	Facets(ctx context.Context, in domain.ListInput) (domain.StatusFacets, error)
	Tags(ctx context.Context, in domain.ListInput) ([]string, error)
	Get(ctx context.Context, id string) (domain.Test, error)
	Create(ctx context.Context, req domain.Request, title string, description *string) (domain.Test, error)
	Update(ctx context.Context, req domain.Request, in domain.UpdateInput) (domain.Test, error)
	Duplicate(ctx context.Context, req domain.Request) (domain.Test, error)
	ListVersions(ctx context.Context, testID string) ([]domain.Version, error)
	Preview(ctx context.Context, testID string, version int) (int, []domain.PreviewQuestion, error)
}

type Publisher interface {
	Publish(ctx context.Context, req domain.PublishRequest) (domain.PublishedVersion, error)
}

// Media resolves a preview question's attachment; nil when object storage is off.
type Media interface {
	Get(ctx context.Context, id string) (mediadomain.Asset, error)
	SignedURL(ctx context.Context, asset mediadomain.Asset) (string, error)
}

type Tests struct {
	tests     Service
	publisher Publisher
	media     Media
}

func NewTests(tests Service, publisher Publisher, media Media) Tests {
	return Tests{tests: tests, publisher: publisher, media: media}
}
