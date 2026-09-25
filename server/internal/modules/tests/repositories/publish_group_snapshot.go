package repositories

import (
	"context"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/content"
	"strings"

	"github.com/jackc/pgx/v5"
)

func freezeSectionGraph(ctx context.Context, tx pgx.Tx, sectionID string, section domain.DraftSection) error {
	questions := make(map[string]string, len(section.Questions))
	for _, question := range section.Questions {
		id, err := freezeQuestion(ctx, tx, sectionID, question)
		if err != nil {
			return err
		}
		questions[strings.ToLower(question.SourceID)] = id
	}
	groups := make(map[string]string, len(section.Groups))
	for _, bundle := range section.Groups {
		id, err := freezeGroup(ctx, tx, sectionID, bundle.Group, questions)
		if err != nil {
			return err
		}
		groups[strings.ToLower(bundle.Group.ID)] = id
	}
	return freezeUnits(ctx, tx, sectionID, section.Units, questions, groups)
}

func freezeUnits(ctx context.Context, tx pgx.Tx, sectionID string, units []domain.DraftUnit, questions, groups map[string]string) error {
	for i, unit := range units {
		var question, group *string
		if unit.QuestionID != "" {
			id, exists := questions[strings.ToLower(unit.QuestionID)]
			if !exists {
				return domain.ErrUnknownQuestion
			}
			question = &id
		} else {
			id, exists := groups[strings.ToLower(unit.GroupID)]
			if !exists {
				return &domain.GroupError{Rule: groupMembershipRule}
			}
			group = &id
		}
		if _, err := tx.Exec(ctx, `INSERT INTO app.test_version_units (test_version_section_id,ordinal,question_id,group_id)
			VALUES ($1,$2,$3,$4)`, sectionID, i, question, group); err != nil {
			return err
		}
	}
	return nil
}

func freezeGroup(ctx context.Context, tx pgx.Tx, sectionID string, group domain.QuestionGroup, questions map[string]string) (string, error) {
	var id string
	if err := tx.QueryRow(ctx, `INSERT INTO app.test_version_groups (test_version_section_id,title,instructions)
		VALUES ($1,$2,$3) RETURNING id::text`, sectionID, group.Title, nullableGroupContent(group.Instructions)).Scan(&id); err != nil {
		return "", err
	}
	for i, member := range group.Members {
		question, exists := questions[strings.ToLower(member.QuestionID)]
		if !exists {
			return "", domain.ErrUnknownQuestion
		}
		if _, err := tx.Exec(ctx, `INSERT INTO app.test_version_group_members (group_id,test_version_section_id,question_id,ordinal,option_order)
			VALUES ($1,$2,$3,$4,$5)`, id, sectionID, question, i, member.OptionOrder); err != nil {
			return "", err
		}
	}
	recordings, err := freezeGroupRecordings(ctx, tx, id, group.Recordings)
	if err != nil {
		return "", err
	}
	for i, material := range group.Stimuli {
		if err := freezeGroupMaterial(ctx, tx, id, i, material, questions, recordings); err != nil {
			return "", err
		}
	}
	return id, nil
}

func freezeGroupRecordings(ctx context.Context, tx pgx.Tx, groupID string, recordings []domain.GroupRecording) (map[string]string, error) {
	byAsset := make(map[string]string, len(recordings))
	for _, recording := range recordings {
		var id string
		if err := tx.QueryRow(ctx, `INSERT INTO app.test_version_group_recordings (group_id,media_asset_id,max_plays,allow_seek,show_transcript_after_submit,transcript)
			VALUES ($1,$2,$3,$4,$5,$6) RETURNING id::text`, groupID, recording.AssetID, recording.Policy.MaxPlays, recording.Policy.AllowSeek, recording.Policy.ShowTranscriptAfterSubmit, recording.Transcript).Scan(&id); err != nil {
			return nil, err
		}
		byAsset[strings.ToLower(recording.AssetID)] = id
	}
	return byAsset, nil
}

func freezeGroupMaterial(ctx context.Context, tx pgx.Tx, groupID string, ordinal int, material domain.GroupStimulus, questions, recordings map[string]string) error {
	var id string
	if err := tx.QueryRow(ctx, `INSERT INTO app.test_version_group_stimuli (group_id,ordinal,title,content)
		VALUES ($1,$2,$3,$4) RETURNING id::text`, groupID, ordinal, material.Title, material.Content).Scan(&id); err != nil {
		return err
	}
	for _, gap := range material.Gaps {
		question, exists := questions[strings.ToLower(gap.QuestionID)]
		if !exists {
			return domain.ErrUnknownQuestion
		}
		if _, err := tx.Exec(ctx, `INSERT INTO app.test_version_group_gap_bindings (stimulus_id,group_id,gap_id,kind,question_id,blank_gap_id)
			VALUES ($1,$2,$3,$4,$5,$6)`, id, groupID, gap.GapID, gap.Kind, question, gap.BlankGapID); err != nil {
			return err
		}
	}
	return freezeGroupAssets(ctx, tx, groupID, id, material, recordings)
}

func freezeGroupAssets(ctx context.Context, tx pgx.Tx, groupID, materialID string, material domain.GroupStimulus, recordings map[string]string) error {
	document, err := content.Parse(material.Content)
	if err != nil {
		return err
	}
	for _, asset := range document.Assets() {
		var recording *string
		if id, exists := recordings[strings.ToLower(asset.ID)]; exists {
			recording = &id
		}
		if _, err := tx.Exec(ctx, `INSERT INTO app.test_version_group_assets (stimulus_id,group_id,media_asset_id,media_asset_kind,recording_id)
			VALUES ($1,$2,$3,$4,$5)`, materialID, groupID, asset.ID, asset.Kind, recording); err != nil {
			return err
		}
	}
	return nil
}
