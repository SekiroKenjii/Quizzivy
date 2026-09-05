package http

import (
	"context"

	mediadomain "quizzivy/internal/modules/media/domain"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/shared/paging"
)

// Service is the slice of the question-bank application this transport needs.
type Service interface {
	List(ctx context.Context, in domain.ListInput) ([]domain.Question, paging.Page, error)
	Facets(ctx context.Context, in domain.ListInput) (domain.TypeFacets, error)
	Get(ctx context.Context, id string) (domain.Question, error)
	Create(ctx context.Context, req domain.WriteRequest) (domain.Question, error)
	Update(ctx context.Context, req domain.WriteRequest) (domain.Question, error)
	Delete(ctx context.Context, req domain.WriteRequest) error
	Duplicate(ctx context.Context, req domain.WriteRequest) (domain.Question, error)
	AddTags(ctx context.Context, ids []string, tags []string) (int, error)
	Tags(ctx context.Context, in domain.ListInput) ([]string, error)
	Counts(ctx context.Context, in domain.ListInput) (int, int, error)
}

// Media resolves a question's attachment for rendering; nil when object storage is off.
type Media interface {
	Get(ctx context.Context, id string) (mediadomain.Asset, error)
	SignedURL(ctx context.Context, asset mediadomain.Asset) (string, error)
}

type Questions struct {
	questions Service
	media     Media
}

func NewQuestions(questions Service, media Media) Questions {
	return Questions{questions: questions, media: media}
}
