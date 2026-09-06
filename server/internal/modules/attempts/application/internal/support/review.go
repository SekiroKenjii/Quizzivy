package support

import (
	"quizzivy/internal/modules/attempts/domain"
)

// Review carries what the review handlers share: their ports and the helpers they call.
type Review struct {
	Repo domain.ReviewRepository
}

func NewReview(repo domain.ReviewRepository) *Review {
	return &Review{Repo: repo}
}
