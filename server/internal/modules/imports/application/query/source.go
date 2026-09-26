package query

import (
	"context"
	"quizzivy/internal/modules/imports/application/worker"
	"quizzivy/internal/modules/imports/domain"
)

type SourceView struct{ ImportID, Role string }

// SourceViewResult is one source's extracted main-body evidence with its original filename.
type SourceViewResult struct {
	Evidence domain.EvidenceDocument
	Filename string
}

type SourceViewHandler struct {
	Repo domain.Repository
	Runs interface {
		DraftRun(context.Context, string) (domain.Run, error)
	}
	Reader worker.EvidenceReader
}

func (h SourceViewHandler) Handle(ctx context.Context, in SourceView) (SourceViewResult, error) {
	current, err := h.Repo.Get(ctx, in.ImportID)
	if err != nil {
		return SourceViewResult{}, err
	}
	if current.FilesRemovedAt != nil {
		return SourceViewResult{}, domain.ErrFilesRemoved
	}
	run, err := h.Runs.DraftRun(ctx, in.ImportID)
	if err != nil {
		return SourceViewResult{}, domain.ErrNoDraft
	}
	evidence, err := h.Reader.Read(ctx, run, in.Role)
	if err != nil {
		return SourceViewResult{}, err
	}
	out := SourceViewResult{Evidence: evidence}
	for _, s := range current.Sources {
		if s.Role == in.Role {
			out.Filename = s.Filename
		}
	}
	return out, nil
}
