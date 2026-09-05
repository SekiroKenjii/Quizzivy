//go:build integration

package application_test

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	questionsdomain "quizzivy/internal/modules/questions/domain"
)

type mediaKinds struct{ pool *pgxpool.Pool }

func (m mediaKinds) Kind(ctx context.Context, assetID string) (string, error) {
	var kind string
	err := m.pool.QueryRow(ctx,
		`SELECT kind::text FROM app.media_assets WHERE id = $1 AND deleted_at IS NULL`, assetID).Scan(&kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", questionsdomain.ErrMediaNotFound
	}
	return kind, err
}
