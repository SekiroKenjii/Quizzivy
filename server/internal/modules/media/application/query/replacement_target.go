package query

import (
	"context"
	"quizzivy/internal/modules/media/application/internal/support"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/shared/access"
)

// ReplacementTarget selects a library asset without reading a replacement body.
type ReplacementTarget struct {
	ID    string
	Scope access.Scope
}
type ReplacementTargetHandler struct{ *support.Service }

func (s ReplacementTargetHandler) Handle(ctx context.Context, in ReplacementTarget) (domain.ReplacementTarget, error) {
	return s.Repo.FindReplacementTarget(ctx, in.Scope, in.ID)
}
