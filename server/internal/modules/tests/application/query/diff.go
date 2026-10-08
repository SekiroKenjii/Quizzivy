package query

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/access"
)

// Diff asks what differs between a version of a test and another paper of it.
type Diff struct {
	TestID  string
	Version int
	Against domain.Against
	Scope   access.Scope
}

// DiffResult is the two sides of a comparison and its changes. From is nil
// when nothing precedes To.
type DiffResult struct {
	From    *domain.DiffSide
	To      domain.DiffSide
	Changes []domain.Change
}

type DiffHandler struct {
	*support.Service
}

func (s DiffHandler) Handle(ctx context.Context, q Diff) (DiffResult, error) {
	papers, err := s.Repo.DiffPapers(ctx, domain.DiffRequest{TestID: q.TestID, Version: q.Version, Against: q.Against, Scope: q.Scope})
	if err != nil {
		return DiffResult{}, err
	}
	result := DiffResult{To: papers.To.Side, Changes: papers.Changes()}
	if papers.From != nil {
		side := papers.From.Side
		result.From = &side
	}
	return result, nil
}
