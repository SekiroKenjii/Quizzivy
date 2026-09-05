package application

import (
	"context"
	"time"

	"quizzivy/internal/modules/tests/domain"
)

// Publisher turns a draft into a new immutable version once the domain's rules pass.
type Publisher struct {
	repo domain.Repository
	now  func() time.Time
}

func NewPublisher(repo domain.Repository) *Publisher {
	return &Publisher{repo: repo, now: time.Now}
}

func (p *Publisher) Publish(ctx context.Context, req domain.PublishRequest) (domain.PublishedVersion, error) {
	return p.repo.Publish(ctx, req, p.now(), domain.Publishing.Validate)
}
