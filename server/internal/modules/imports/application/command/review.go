package command

import (
	"context"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/shared/actor"
)

type SaveReview struct {
	ImportID         string
	ExpectedRevision int64
	Title            string
	Sections         []domain.DraftSection
	Acknowledged     []string
	Actor            actor.Actor
}

type SaveReviewHandler struct{ Drafts domain.Drafts }

func (h SaveReviewHandler) Handle(ctx context.Context, in SaveReview) (domain.ReviewState, error) {
	stored, err := h.Drafts.Draft(ctx, in.ImportID)
	if err != nil {
		return domain.ReviewState{}, err
	}
	next := domain.Draft{Version: domain.DraftVersion, Title: in.Title, Sections: in.Sections, Notices: stored.Draft.Notices, Acknowledged: in.Acknowledged}
	if next.Sections == nil {
		next.Sections = []domain.DraftSection{}
	}
	if next.Acknowledged == nil {
		next.Acknowledged = []string{}
	}
	if err := domain.ValidateEdit(next); err != nil {
		return domain.ReviewState{}, err
	}
	saved, err := h.Drafts.SaveDraft(ctx, domain.SaveDraft{ImportID: in.ImportID, ExpectedRevision: in.ExpectedRevision, Draft: next, Actor: in.Actor})
	if err != nil {
		return domain.ReviewState{}, err
	}
	return domain.ReviewState{Draft: saved, Review: domain.Assess(saved.Draft)}, nil
}
