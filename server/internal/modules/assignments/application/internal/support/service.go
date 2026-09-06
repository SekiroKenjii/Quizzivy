package support

import (
	"quizzivy/internal/modules/assignments/domain"
)

// Service carries what the service handlers share: their ports and the helpers they call.
type Service struct {
	Repo domain.Repository
}

func NewService(repo domain.Repository) *Service {
	return &Service{Repo: repo}
}
