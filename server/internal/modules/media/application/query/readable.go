package query

import (
	"context"
	"quizzivy/internal/modules/media/application/internal/support"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/shared/access"
)

// Readable asks which of IDs the scope may read, and so bind: the kind of each
// readable id, keyed by its canonical text. An id missing from the answer is
// missing, deleted or not the scope's to read, and must be answered alike.
type Readable struct {
	Scope access.Scope
	IDs   []string
}

type ReadableHandler struct {
	*support.Service
}

func (s ReadableHandler) Handle(ctx context.Context, q Readable) (map[string]domain.Kind, error) {
	return s.Repo.Readable(ctx, q.Scope, q.IDs)
}
