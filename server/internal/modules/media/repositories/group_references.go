package repositories

import (
	"context"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/platform/db"

	"github.com/jackc/pgx/v5"
)

// GroupReferences names every independent context protecting an asset, including archived groups and member-only media.
func GroupReferences(ctx context.Context, q db.Querier, assetID string) ([]domain.GroupRef, error) {
	rows, err := q.Query(ctx, `WITH referenced_groups AS (
		SELECT group_id FROM app.group_stimulus_assets WHERE media_asset_id=$1
		UNION
		SELECT group_id FROM app.group_recordings WHERE media_asset_id=$1
		UNION
		SELECT context_group_id FROM app.questions WHERE media_asset_id=$1 AND context_group_id IS NOT NULL
	)
	SELECT g.id::text,g.title,s.test_id::text
	FROM referenced_groups r JOIN app.question_groups g ON g.id=r.group_id
	LEFT JOIN app.test_sections s ON s.id=g.owner_section_id
	ORDER BY g.title,g.id`, assetID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[domain.GroupRef])
}
