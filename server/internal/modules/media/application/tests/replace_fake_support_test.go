package application_test

import (
	"context"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/shared/access"
)

func (r *scriptedRepo) FindReplacementTarget(ctx context.Context, scope access.Scope, id string) (domain.ReplacementTarget, error) {
	a, err := r.Find(ctx, scope, id)
	return domain.ReplacementTarget{Asset: a, OwnerID: scope.UserID}, err
}
func (r *scriptedRepo) Replace(context.Context, domain.ReplaceInput) (domain.ReplaceResult, error) {
	return domain.ReplaceResult{}, domain.ErrNotFound
}
