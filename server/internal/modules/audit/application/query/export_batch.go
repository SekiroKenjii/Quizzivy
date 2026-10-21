package query

import (
	"context"

	"quizzivy/internal/modules/audit/application/internal/support"
	"quizzivy/internal/modules/audit/domain"
)

// ExportBatch reads the next domain.ExportBatchSize entries of an export plan
// after After, or the first when After is nil, newest first.
type ExportBatch struct {
	Filter domain.Filter
	After  *domain.Cursor
}

// ExportBatchResult is a batch of an export and where the one after it starts;
// Next is nil when this was the last.
type ExportBatchResult struct {
	Entries []domain.Entry
	Next    *domain.Cursor
}

type ExportBatchHandler struct {
	*support.Service
}

func (s ExportBatchHandler) Handle(ctx context.Context, q ExportBatch) (ExportBatchResult, error) {
	records, err := s.Repo.ExportBatch(ctx, q.Filter, q.After, domain.ExportBatchSize)
	if err != nil {
		return ExportBatchResult{}, err
	}
	entries, err := s.Present(ctx, records)
	if err != nil {
		return ExportBatchResult{}, err
	}
	result := ExportBatchResult{Entries: entries}
	if len(records) == domain.ExportBatchSize {
		last := records[len(records)-1]
		result.Next = &domain.Cursor{OccurredAt: last.OccurredAt, ID: last.ID}
	}
	return result, nil
}
