package support

import (
	"quizzivy/internal/modules/attempts/domain"
)

// Integrity carries what the integrity handlers share: their ports and the helpers they call.
type Integrity struct {
	Repo domain.TimelineRepository
}

func NewIntegrity(repo domain.TimelineRepository) *Integrity {
	return &Integrity{Repo: repo}
}
