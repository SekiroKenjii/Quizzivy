package support

import (
	"quizzivy/internal/modules/tests/domain"
	"time"
)

// Publisher carries what the publisher handlers share: their ports and the helpers they call.
type Publisher struct {
	Repo domain.Repository
	Now  func() time.Time
}

func NewPublisher(repo domain.Repository) *Publisher {
	return &Publisher{Repo: repo, Now: time.Now}
}
