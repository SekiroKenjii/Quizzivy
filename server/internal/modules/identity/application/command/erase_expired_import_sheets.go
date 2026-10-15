package command

import (
	"context"
	"errors"

	"quizzivy/internal/modules/identity/application/internal/support"
)

// EraseExpiredImportSheets clears the sealed passwords of at most Limit rows
// of hand-out sheets that were not downloaded within their 24 hours, and
// returns how many it cleared. It is intended for a scheduled call, repeated
// while it returns Limit.
type EraseExpiredImportSheets struct {
	Limit int
}

// EraseExpiredImportSheetsHandler erases through the import store.
type EraseExpiredImportSheetsHandler struct {
	*support.Imports
}

func (h EraseExpiredImportSheetsHandler) Handle(ctx context.Context, c EraseExpiredImportSheets) (int, error) {
	if c.Limit <= 0 {
		return 0, errors.New("erase expired import sheets: limit must be positive")
	}
	return h.Repo.EraseExpiredSheets(ctx, h.Service.Now(), c.Limit)
}
