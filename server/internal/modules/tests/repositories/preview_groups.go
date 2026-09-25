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
			group := domain.PreviewGroup{QuestionIDs: []string{}, Stimuli: []domain.GroupStimulus{}, Recordings: []domain.PreviewRecording{}, AssetIDs: []string{}}
			err := rows.Scan(&group.ID, &group.SectionID, &group.Title, &group.Instructions)
			return group, err
		})
	if err != nil || len(groups) == 0 {
		return groups, err
	}
	b := previewBatch{groups: groups, position: make(map[string]int, len(groups)), stimulus: map[string][2]int{}}
	ids := make([]string, len(groups))
	for i, g := range groups {
		ids[i] = g.ID
		b.position[g.ID] = i
	}
	for _, load := range []func(context.Context, pgx.Tx, []string) error{b.members, b.stimuli, b.recordings} {
		if err := load(ctx, tx, ids); err != nil {
			return nil, err
		}
	}
	stimulusIDs := make([]string, 0, len(b.stimulus))
	for id := range b.stimulus {
		stimulusIDs = append(stimulusIDs, id)
	}
	if err := b.gaps(ctx, tx, stimulusIDs); err != nil {
		return nil, err
	}
	return groups, b.assets(ctx, tx, stimulusIDs)
}

type previewBatch struct {
	groups   []domain.PreviewGroup
	position map[string]int
	stimulus map[string][2]int
}

func (b previewBatch) members(ctx context.Context, tx pgx.Tx, ids []string) error {
	return eachRow(ctx, tx, `SELECT group_id::text,question_id::text FROM app.test_version_group_members
        WHERE group_id=ANY($1::uuid[]) ORDER BY group_id,ordinal`, ids, func(rows pgx.Rows) error {
		var group, question string
		if err := rows.Scan(&group, &question); err != nil {
			return err
		}
		g := &b.groups[b.position[group]]
		g.QuestionIDs = append(g.QuestionIDs, question)
		return nil
	})
}

func (b previewBatch) stimuli(ctx context.Context, tx pgx.Tx, ids []string) error {
	return eachRow(ctx, tx, `SELECT group_id::text,id::text,title,content FROM app.test_version_group_stimuli
        WHERE group_id=ANY($1::uuid[]) ORDER BY group_id,ordinal`, ids, func(rows pgx.Rows) error {
		var group string
		material := domain.GroupStimulus{Gaps: []domain.GroupGapBinding{}}
		if err := rows.Scan(&group, &material.ID, &material.Title, &material.Content); err != nil {
			return err
		}
		index := b.position[group]
		g := &b.groups[index]
		b.stimulus[material.ID] = [2]int{index, len(g.Stimuli)}
		g.Stimuli = append(g.Stimuli, material)
		return nil
	})
}

func (b previewBatch) gaps(ctx context.Context, tx pgx.Tx, stimulusIDs []string) error {
	return eachRow(ctx, tx, `SELECT stimulus_id::text,kind,gap_id,question_id::text,blank_gap_id FROM app.test_version_group_gap_bindings
        WHERE stimulus_id=ANY($1::uuid[]) ORDER BY stimulus_id,gap_id`, stimulusIDs, func(rows pgx.Rows) error {
		var id string
		var gap domain.GroupGapBinding
		if err := rows.Scan(&id, &gap.Kind, &gap.GapID, &gap.QuestionID, &gap.BlankGapID); err != nil {
			return err
		}
		at, exists := b.stimulus[id]
		if !exists {
			return &domain.GroupError{Rule: "group_gap", StimulusID: id}
		}
		material := &b.groups[at[0]].Stimuli[at[1]]
		material.Gaps = append(material.Gaps, gap)
		return nil
	})
}

func (b previewBatch) recordings(ctx context.Context, tx pgx.Tx, ids []string) error {
	return eachRow(ctx, tx, `SELECT group_id::text,id::text,media_asset_id::text,max_plays,allow_seek,show_transcript_after_submit
        FROM app.test_version_group_recordings WHERE group_id=ANY($1::uuid[]) ORDER BY group_id,id`, ids, func(rows pgx.Rows) error {
		var group string
		var recording domain.PreviewRecording
		if err := rows.Scan(&group, &recording.ID, &recording.AssetID, &recording.Policy.MaxPlays, &recording.Policy.AllowSeek, &recording.Policy.ShowTranscriptAfterSubmit); err != nil {
			return err
		}
		g := &b.groups[b.position[group]]
		g.Recordings = append(g.Recordings, recording)
		return nil
	})
}

func (b previewBatch) assets(ctx context.Context, tx pgx.Tx, stimulusIDs []string) error {
	return eachRow(ctx, tx, `SELECT DISTINCT s.group_id::text,a.media_asset_id::text FROM app.test_version_group_assets a
        JOIN app.test_version_group_stimuli s ON s.id=a.stimulus_id WHERE a.stimulus_id=ANY($1::uuid[]) ORDER BY 1,2`, stimulusIDs, func(rows pgx.Rows) error {
		var group, asset string
		if err := rows.Scan(&group, &asset); err != nil {
			return err
		}
		g := &b.groups[b.position[group]]
		g.AssetIDs = append(g.AssetIDs, asset)
		return nil
	})
}

func eachRow(ctx context.Context, tx pgx.Tx, sql string, ids []string, scan func(pgx.Rows) error) error {
	if len(ids) == 0 {
		return nil
	}
	rows, err := tx.Query(ctx, sql, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}
