package repositories

import (
	"context"
	"quizzivy/internal/modules/tests/domain"

	"github.com/jackc/pgx/v5"
)

func readGroupMaterials(ctx context.Context, tx pgx.Tx, group *domain.QuestionGroup) error {
	rows, err := tx.Query(ctx, `SELECT id::text, title, content FROM app.group_stimuli WHERE group_id=$1 ORDER BY ordinal`, group.ID)
	if err != nil {
		return err
	}
	group.Stimuli, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.GroupStimulus, error) {
		material := domain.GroupStimulus{Gaps: []domain.GroupGapBinding{}}
		err := row.Scan(&material.ID, &material.Title, &material.Content)
		return material, err
	})
	if err != nil {
		return err
	}
	byID := make(map[string]int)
	for i, material := range group.Stimuli {
		byID[material.ID] = i
	}
	rows, err = tx.Query(ctx, `SELECT stimulus_id::text, kind, gap_id, question_id::text, blank_gap_id
		FROM app.group_gap_bindings WHERE group_id=$1 ORDER BY stimulus_id, gap_id`, group.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var gap domain.GroupGapBinding
		if err := rows.Scan(&id, &gap.Kind, &gap.GapID, &gap.QuestionID, &gap.BlankGapID); err != nil {
			return err
		}
		index, exists := byID[id]
		if !exists {
			return &domain.GroupError{Rule: "group_gap", StimulusID: id}
		}
		group.Stimuli[index].Gaps = append(group.Stimuli[index].Gaps, gap)
	}
	return rows.Err()
}

func readGroupRecordings(ctx context.Context, tx pgx.Tx, group *domain.QuestionGroup) error {
	rows, err := tx.Query(ctx, `SELECT id::text, media_asset_id::text, max_plays, allow_seek,
		show_transcript_after_submit, transcript FROM app.group_recordings WHERE group_id=$1 ORDER BY id`, group.ID)
	if err != nil {
		return err
	}
	group.Recordings, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.GroupRecording, error) {
		var recording domain.GroupRecording
		err := row.Scan(&recording.ID, &recording.AssetID, &recording.Policy.MaxPlays,
			&recording.Policy.AllowSeek, &recording.Policy.ShowTranscriptAfterSubmit, &recording.Transcript)
		return recording, err
	})
	return err
}
