package command

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/shared/actor"

	"github.com/google/uuid"
)

type Commit struct {
	ImportID, RequestID string
	DraftRevision       int64
	Actor               actor.Actor
}

type CommitResult struct {
	TestID string
	Import domain.Import
}

type CommitHandler struct {
	Repo         domain.Repository
	Drafts       domain.Drafts
	Materializer ports.Materializer
}

func (h CommitHandler) Handle(ctx context.Context, in Commit) (CommitResult, error) {
	if result, done, err := h.replay(ctx, in); done || err != nil {
		return result, err
	}
	stored, err := h.Drafts.Draft(ctx, in.ImportID)
	if err != nil {
		return CommitResult{}, err
	}
	if stored.Status != "needs_review" {
		return CommitResult{}, domain.ErrConflict
	}
	if stored.Revision != in.DraftRevision {
		return CommitResult{}, domain.ErrStale
	}
	plan, err := domain.Plan(stored.Draft, stored.Title, uuid.NewString)
	if err != nil {
		return CommitResult{}, err
	}
	body, err := json.Marshal(stored.Draft)
	if err != nil {
		return CommitResult{}, err
	}
	digest := sha256.Sum256(body)
	_, err = h.Materializer.Materialize(ctx, plan, in.Actor, func(ctx context.Context, store domain.CommitStore, testID string) error {
		return store.RecordCommit(ctx, domain.CommitRecord{Commit: domain.Commit{ImportID: in.ImportID, RequestID: in.RequestID, DraftRevision: in.DraftRevision, Digest: digest[:], TestID: &testID}, Actor: in.Actor})
	})
	if errors.Is(err, domain.ErrConflict) {
		if result, done, replayErr := h.replay(ctx, in); done || replayErr != nil {
			return result, replayErr
		}
	}
	if err != nil {
		return CommitResult{}, err
	}
	result, _, err := h.replay(ctx, in)
	return result, err
}

func (h CommitHandler) replay(ctx context.Context, in Commit) (CommitResult, bool, error) {
	previous, err := h.Drafts.Commit(ctx, in.ImportID)
	if errors.Is(err, domain.ErrNotFound) {
		return CommitResult{}, false, nil
	}
	if err != nil {
		return CommitResult{}, false, err
	}
	if previous.RequestID != in.RequestID || previous.TestID == nil {
		return CommitResult{}, false, domain.ErrConflict
	}
	current, err := h.Repo.Get(ctx, in.ImportID)
	return CommitResult{TestID: *previous.TestID, Import: current}, true, err
}
