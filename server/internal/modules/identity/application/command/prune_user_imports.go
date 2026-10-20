package command

import (
	"context"
	"errors"

	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
)

// PruneUserImports deletes at most Limit user imports, with their rows, that
// finished more than domain.UserImportsKept ago, and returns how many jobs
// went. It is intended for a scheduled call, repeated while it returns Limit.
type PruneUserImports struct {
	Limit int
}

// PruneUserImportsHandler prunes through the retention store.
type PruneUserImportsHandler struct {
	*support.Service
	Imports domain.UserImportRetention
}

func (h PruneUserImportsHandler) Handle(ctx context.Context, c PruneUserImports) (int, error) {
	if c.Limit <= 0 {
		return 0, errors.New("prune user imports: limit must be positive")
	}
	return h.Imports.Prune(ctx, h.Now().Add(-domain.UserImportsKept), c.Limit)
}
