package query

import (
	"context"
	"errors"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/access"
)

type Get struct {
	ID    string
	Scope access.Scope
}

type GetHandler struct {
	*support.Service
}

// Handle returns the test with UnpublishedChanges counted against its latest
// version. The count is a second, read-only transaction after the one that
// read the test; it stays nil for a test never published and for a draft or a
// latest version whose groups are refused, and any other failure of it fails
// the read.
func (s GetHandler) Handle(ctx context.Context, q Get) (domain.Test, error) {
	test, err := s.Repo.Get(ctx, q.Scope, q.ID)
	if err != nil || test.CurrentVersion == 0 {
		return test, err
	}
	papers, err := s.Repo.DiffPapers(ctx, domain.DiffRequest{TestID: q.ID, Against: domain.Against{Kind: domain.AgainstDraft}, Scope: q.Scope})
	switch {
	case err == nil:
		changes, err := papers.Changes()
		if err != nil {
			return domain.Test{}, err
		}
		count := len(changes)
		test.UnpublishedChanges = &count
	case errors.Is(err, domain.ErrNotPublished), errors.Is(err, domain.ErrDraftUnreadable), errors.Is(err, domain.ErrVersionUnreadable):
	default:
		return domain.Test{}, err
	}
	return test, nil
}
