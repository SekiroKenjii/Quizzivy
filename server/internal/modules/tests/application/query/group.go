package query

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/access"
)

// Group reads a complete editable graph and its observed revisions.
type Group struct {
	ID    string
	Scope access.Scope
}
type GroupHandler struct{ *support.Groups }

func (s GroupHandler) Handle(ctx context.Context, q Group) (domain.StoredGroup, error) {
	if s.Repo == nil {
		return domain.StoredGroup{}, domain.ErrGroupUnavailable
	}
	return s.Repo.Get(ctx, q.Scope, q.ID)
}
