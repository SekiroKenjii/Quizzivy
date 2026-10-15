package query

import (
	"context"

	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
)

// GetUserImport asks for one import of the caller's.
type GetUserImport struct {
	Request domain.WriteRequest
	ID      string
}

// GetUserImportResult is the job with what the caller may do about its sheet.
// Sheet is nil for a job delivered by email.
type GetUserImportResult struct {
	Job   domain.ImportJob
	Sheet *domain.ImportSheetState
}

type GetUserImportHandler struct {
	*support.Imports
}

// Handle fails with domain.ErrUserImportNotFound for an import that does not
// exist or is another user's.
func (h GetUserImportHandler) Handle(ctx context.Context, q GetUserImport) (GetUserImportResult, error) {
	job, err := h.Repo.Find(ctx, q.Request.ActorID, q.ID)
	if err != nil {
		return GetUserImportResult{}, err
	}
	out := GetUserImportResult{Job: job}
	if state, ok := job.Sheet(h.Service.Now()); ok {
		out.Sheet = &state
	}
	return out, nil
}
