package repositories

import (
	"context"
	"errors"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/db"

	"github.com/jackc/pgx/v5"
)

// GroupContexts reads one coherent frozen context without loading keys or transcripts.
func (s *Postgres) GroupContexts(ctx context.Context, versionID string) ([]domain.PreviewGroup, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var id string
	if err := tx.QueryRow(ctx, `SELECT id::text FROM app.test_versions WHERE id=$1`, versionID).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	groups, err := previewGroups(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	return groups, tx.Commit(ctx)
}

func previewGroups(ctx context.Context, tx pgx.Tx, versionID string) ([]domain.PreviewGroup, error) {
	groups, err := db.QueryMany(ctx, tx, `SELECT g.id::text,g.test_version_section_id::text,g.title,g.instructions
        FROM app.test_version_groups g JOIN app.test_version_sections s ON s.id=g.test_version_section_id
        JOIN app.test_version_units u ON u.group_id=g.id
        WHERE s.test_version_id=$1 ORDER BY s.ordinal,u.ordinal`, []any{versionID},
		func(rows pgx.Rows) (domain.PreviewGroup, error) {
			var group domain.PreviewGroup
			err := rows.Scan(&group.ID, &group.SectionID, &group.Title, &group.Instructions)
			return group, err
		})
	if err != nil {
		return nil, err
	}
	for i := range groups {
		if err := previewGroupContext(ctx, tx, &groups[i]); err != nil {
			return nil, err
		}
	}
	return groups, nil
}

func previewGroupContext(ctx context.Context, tx pgx.Tx, group *domain.PreviewGroup) error {
	var err error
	group.QuestionIDs, err = db.QueryMany(ctx, tx, `SELECT question_id::text FROM app.test_version_group_members
        WHERE group_id=$1 ORDER BY ordinal`, []any{group.ID}, func(rows pgx.Rows) (string, error) {
		var id string
		err := rows.Scan(&id)
		return id, err
	})
	if err != nil {
		return err
	}
	materials := domain.QuestionGroup{ID: group.ID}
	if err := readGroupMaterials(ctx, tx, &materials, frozenGraphTables); err != nil {
		return err
	}
	group.Stimuli = materials.Stimuli
	group.Recordings, err = db.QueryMany(ctx, tx, `SELECT id::text,media_asset_id::text,max_plays,allow_seek,show_transcript_after_submit
        FROM app.test_version_group_recordings WHERE group_id=$1 ORDER BY id`, []any{group.ID},
		func(rows pgx.Rows) (domain.PreviewRecording, error) {
			var recording domain.PreviewRecording
			err := rows.Scan(&recording.ID, &recording.AssetID, &recording.Policy.MaxPlays, &recording.Policy.AllowSeek, &recording.Policy.ShowTranscriptAfterSubmit)
			return recording, err
		})
	if err != nil {
		return err
	}
	group.AssetIDs, err = db.QueryMany(ctx, tx, `SELECT DISTINCT media_asset_id::text FROM app.test_version_group_assets
        WHERE group_id=$1 ORDER BY media_asset_id::text`, []any{group.ID}, func(rows pgx.Rows) (string, error) {
		var id string
		err := rows.Scan(&id)
		return id, err
	})
	return err
}
