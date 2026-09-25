package repositories

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/content"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (s *GroupsPostgres) lockGroupAssets(ctx context.Context, tx pgx.Tx, bundle domain.GroupBundle) error {
	assets := make(map[string]bool)
	for _, question := range bundle.Questions {
		if question.Input.MediaAssetID != nil {
			assets[strings.ToLower(*question.Input.MediaAssetID)] = true
		}
	}
	for _, material := range bundle.Group.Stimuli {
		document, err := content.Parse(material.Content)
		if err != nil {
			return err
		}
		for _, asset := range document.Assets() {
			assets[strings.ToLower(asset.ID)] = true
		}
	}
	ids := make([]string, 0, len(assets))
	for id := range assets {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	if len(ids) > 0 && s.media == nil {
		return fmt.Errorf("groups: media reference locks unavailable")
	}
	for _, id := range ids {
		if err := s.media.LockForVersionUse(ctx, tx, id); err != nil {
			return err
		}
	}
	return nil
}

func insertGroupRecordings(ctx context.Context, tx pgx.Tx, group domain.QuestionGroup) error {
	for _, recording := range group.Recordings {
		_, err := tx.Exec(ctx, `INSERT INTO app.group_recordings (id,group_id,media_asset_id,max_plays,allow_seek,show_transcript_after_submit,transcript)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`, recording.ID, group.ID, recording.AssetID, recording.Policy.MaxPlays, recording.Policy.AllowSeek, recording.Policy.ShowTranscriptAfterSubmit, recording.Transcript)
		if err != nil {
			return err
		}
	}
	return nil
}

func insertGroupMaterials(ctx context.Context, tx pgx.Tx, group domain.QuestionGroup) error {
	recordings := groupRecordingIDs(group)
	for i, material := range group.Stimuli {
		if _, err := tx.Exec(ctx, `INSERT INTO app.group_stimuli (id,group_id,ordinal,title,content) VALUES ($1,$2,$3,$4,$5)`, material.ID, group.ID, i, material.Title, material.Content); err != nil {
			return err
		}
		if err := insertMaterialGaps(ctx, tx, group.ID, material); err != nil {
			return err
		}
		if err := insertMaterialAssets(ctx, tx, group.ID, material, recordings); err != nil {
			return err
		}
	}
	return nil
}

func insertMaterialGaps(ctx context.Context, tx pgx.Tx, groupID string, material domain.GroupStimulus) error {
	for _, gap := range material.Gaps {
		_, err := tx.Exec(ctx, `INSERT INTO app.group_gap_bindings (stimulus_id,group_id,gap_id,kind,question_id,blank_gap_id)
			VALUES ($1,$2,$3,$4,$5,$6)`, material.ID, groupID, gap.GapID, gap.Kind, gap.QuestionID, gap.BlankGapID)
		if err != nil {
			return err
		}
	}
	return nil
}

func insertMaterialAssets(ctx context.Context, tx pgx.Tx, groupID string, material domain.GroupStimulus, recordings map[string]string) error {
	document, err := content.Parse(material.Content)
	if err != nil {
		return err
	}
	for _, asset := range document.Assets() {
		var recording *string
		if id, exists := recordings[strings.ToLower(asset.ID)]; exists {
			recording = &id
		}
		if _, err := tx.Exec(ctx, `INSERT INTO app.group_stimulus_assets (stimulus_id,group_id,media_asset_id,media_asset_kind,recording_id)
			VALUES ($1,$2,$3,$4,$5)`, material.ID, groupID, asset.ID, asset.Kind, recording); err != nil {
			return err
		}
	}
	return nil
}

func groupRecordingIDs(group domain.QuestionGroup) map[string]string {
	recordings := make(map[string]string)
	for _, recording := range group.Recordings {
		recordings[strings.ToLower(recording.AssetID)] = recording.ID
	}
	return recordings
}

func nullableGroupContent(raw json.RawMessage) any {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil
	}
	return raw
}

func checkGroupAssetBindings(ctx context.Context, tx pgx.Tx, group domain.QuestionGroup, tables groupGraphTables) error {
	expected := make(map[string]string)
	recordings := groupRecordingIDs(group)
	for _, material := range group.Stimuli {
		document, err := content.Parse(material.Content)
		if err != nil {
			return err
		}
		for _, asset := range document.Assets() {
			id := strings.ToLower(asset.ID)
			expected[strings.ToLower(material.ID)+":"+id] = asset.Kind + ":" + recordings[id]
		}
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT stimulus_id::text, media_asset_id::text, media_asset_kind::text, coalesce(recording_id::text,'')
		FROM %s WHERE group_id=$1`, tables.assets), group.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var material, asset, kind, recording string
		if err := rows.Scan(&material, &asset, &kind, &recording); err != nil {
			return err
		}
		key := material + ":" + asset
		if expected[key] != kind+":"+recording {
			return &domain.GroupError{Rule: "group_asset", StimulusID: material}
		}
		delete(expected, key)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(expected) != 0 {
		return &domain.GroupError{Rule: "group_asset"}
	}
	return nil
}
