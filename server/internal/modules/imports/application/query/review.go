package query

import (
	"context"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/shared/access"
)

// Review reads the review copy of an import Scope reaches; another creator's
// answers ErrNotFound before any other refusal.
type Review struct {
	ImportID string
	Scope    access.Scope
}

type ReviewHandler struct {
	Repo   domain.Repository
	Drafts domain.Drafts
}

func (h ReviewHandler) Handle(ctx context.Context, in Review) (domain.ReviewState, error) {
	parent, err := h.Repo.Get(ctx, in.Scope, in.ImportID)
	if err != nil {
		return domain.ReviewState{}, err
	}
	if parent.FilesRemovedAt != nil {
		return domain.ReviewState{}, domain.ErrFilesRemoved
	}
	stored, err := h.Drafts.Draft(ctx, in.Scope, in.ImportID)
	if err != nil {
		return domain.ReviewState{}, err
	}
	return domain.ReviewState{Draft: stored, Review: domain.Assess(stored.Draft)}, nil
}
