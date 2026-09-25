package query

import (
	"context"
	"quizzivy/internal/modules/imports/domain"
)

type Review struct{ ImportID string }

type ReviewHandler struct{ Drafts domain.Drafts }

func (h ReviewHandler) Handle(ctx context.Context, in Review) (domain.ReviewState, error) {
	stored, err := h.Drafts.Draft(ctx, in.ImportID)
	if err != nil {
		return domain.ReviewState{}, err
	}
	return domain.ReviewState{Draft: stored, Review: domain.Assess(stored.Draft)}, nil
}
