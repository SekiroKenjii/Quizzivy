package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/attempts/domain"
	testsdomain "quizzivy/internal/modules/tests/domain"

	"github.com/jackc/pgx/v5"
)

func (s *Postgres) DeliveryVersion(ctx context.Context, versionID string) (testsdomain.DeliveryVersion, error) {
	var version testsdomain.DeliveryVersion
	err := s.QueryRow(ctx, `SELECT delivery_version FROM app.test_versions WHERE id=$1::uuid`, versionID).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("attempts: read delivery version: %w", err)
	}
	return version, nil
}
