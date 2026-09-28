//go:build integration

package application_test

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	mediarepo "quizzivy/internal/modules/media/repositories"
	questionsdomain "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/shared/access"
)

type mediaKinds struct{ pool *pgxpool.Pool }

func (m mediaKinds) Kind(ctx context.Context, scope access.Scope, assetID string) (string, error) {
	id := strings.ToLower(assetID)
	kinds, err := mediarepo.Readable(ctx, m.pool, scope, []string{id})
	if err != nil {
		return "", err
	}
	kind, ok := kinds[id]
	if !ok {
		return "", questionsdomain.ErrMediaNotFound
	}
	return string(kind), nil
}
