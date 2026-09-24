package repositories

import (
	"context"
	"errors"
	questions "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/tests/domain"

	"github.com/jackc/pgx/v5"
)

// Frozen reads a complete immutable group, including teacher-only keys and transcripts, while protecting its parent version from deletion.
func (s *GroupsPostgres) Frozen(ctx context.Context, id string) (domain.GroupBundle, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.GroupBundle{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	bundle, err := readFrozenGroup(ctx, tx, id)
	if err != nil {
		return domain.GroupBundle{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.GroupBundle{}, err
	}
	return bundle, nil
}

func readFrozenGroup(ctx context.Context, tx pgx.Tx, id string) (domain.GroupBundle, error) {
	var bundle domain.GroupBundle
	err := tx.QueryRow(ctx, `SELECT g.id::text,g.title,g.instructions FROM app.test_version_groups g
		JOIN app.test_version_sections s ON s.id=g.test_version_section_id
		JOIN app.test_versions v ON v.id=s.test_version_id WHERE g.id=$1 FOR SHARE OF v`, id).
		Scan(&bundle.Group.ID, &bundle.Group.Title, &bundle.Group.Instructions)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.GroupBundle{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.GroupBundle{}, err
	}
	if err := readFrozenMembers(ctx, tx, &bundle); err != nil {
		return domain.GroupBundle{}, err
	}
	if err := readGroupMaterials(ctx, tx, &bundle.Group, frozenGraphTables); err != nil {
		return domain.GroupBundle{}, err
	}
	if err := readGroupRecordings(ctx, tx, &bundle.Group, frozenGraphTables); err != nil {
		return domain.GroupBundle{}, err
	}
	if err := checkGroupAssetBindings(ctx, tx, bundle.Group, frozenGraphTables); err != nil {
		return domain.GroupBundle{}, err
	}
	return bundle, bundle.ValidateForPublish(false)
}

func readFrozenMembers(ctx context.Context, tx pgx.Tx, bundle *domain.GroupBundle) error {
	rows, err := tx.Query(ctx, `SELECT q.id::text,q.type::text,q.prompt,q.prompt_content,q.explanation_content,q.points::text,
		q.media_asset_id::text,q.media_asset_kind::text,q.audio_max_plays,q.audio_allow_seek,q.audio_show_transcript_after,
		q.transcript,q.explanation,q.sample_answer,m.ordinal,m.option_order
		FROM app.test_version_group_members m JOIN app.test_version_questions q ON q.id=m.question_id
		WHERE m.group_id=$1 ORDER BY m.ordinal`, bundle.Group.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	byID := make(map[string]int)
	for rows.Next() {
		var question domain.GroupQuestion
		var member domain.GroupMember
		var ordinal int
		var allow, show *bool
		var maxPlays *int
		in := &question.Input
		if err := rows.Scan(&question.ID, &in.Type, &in.Prompt, &in.PromptContent, &in.ExplanationContent, &in.Points,
			&in.MediaAssetID, &question.MediaAssetKind, &maxPlays, &allow, &show, &in.Transcript, &in.Explanation, &in.SampleAnswer, &ordinal, &member.OptionOrder); err != nil {
			return err
		}
		if ordinal != len(bundle.Questions) {
			return &domain.GroupError{Rule: groupMembershipRule}
		}
		in.Tags = []string{}
		if allow != nil && show != nil {
			in.Audio = &questions.AudioPolicy{MaxPlays: maxPlays, AllowSeek: *allow, ShowTranscriptAfterSubmit: *show}
		}
		member.QuestionID = question.ID
		byID[question.ID] = len(bundle.Questions)
		bundle.Group.Members = append(bundle.Group.Members, member)
		bundle.Questions = append(bundle.Questions, question)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := readFrozenOptions(ctx, tx, bundle, byID); err != nil {
		return err
	}
	return readFrozenBlanks(ctx, tx, bundle, byID)
}

func readFrozenOptions(ctx context.Context, tx pgx.Tx, bundle *domain.GroupBundle, byID map[string]int) error {
	rows, err := tx.Query(ctx, `SELECT o.test_version_question_id::text,o.id::text,o.text,o.content,o.is_correct
		FROM app.test_version_options o JOIN app.test_version_group_members m ON m.question_id=o.test_version_question_id
		WHERE m.group_id=$1 ORDER BY o.test_version_question_id,o.ordinal`, bundle.Group.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var option questions.OptionInput
		if err := rows.Scan(&id, &option.ID, &option.Text, &option.Content, &option.IsCorrect); err != nil {
			return err
		}
		index, exists := byID[id]
		if !exists {
			return &domain.GroupError{Rule: groupMembershipRule}
		}
		bundle.Questions[index].Input.Options = append(bundle.Questions[index].Input.Options, option)
	}
	return rows.Err()
}

func readFrozenBlanks(ctx context.Context, tx pgx.Tx, bundle *domain.GroupBundle, byID map[string]int) error {
	rows, err := tx.Query(ctx, `SELECT b.test_version_question_id::text,b.id::text,b.ordinal,b.gap_id,b.case_sensitive,
		coalesce(array_agg(a.answer ORDER BY a.answer) FILTER (WHERE a.answer IS NOT NULL),'{}')
		FROM app.test_version_blanks b JOIN app.test_version_group_members m ON m.question_id=b.test_version_question_id
		LEFT JOIN app.test_version_blank_answers a ON a.test_version_blank_id=b.id WHERE m.group_id=$1
		GROUP BY b.id ORDER BY b.test_version_question_id,b.ordinal`, bundle.Group.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var blank questions.BlankInput
		if err := rows.Scan(&id, &blank.ID, &blank.Ordinal, &blank.GapID, &blank.CaseSensitive, &blank.AcceptedAnswers); err != nil {
			return err
		}
		index, exists := byID[id]
		if !exists {
			return &domain.GroupError{Rule: groupMembershipRule}
		}
		bundle.Questions[index].Input.Blanks = append(bundle.Questions[index].Input.Blanks, blank)
	}
	return rows.Err()
}
