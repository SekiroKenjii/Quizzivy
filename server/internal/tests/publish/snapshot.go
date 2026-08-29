package publish

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/media"
)

// snapshot freezes the draft into the version tables.
//
// This is what makes editing a published test safe: an attempt renders from
// these rows, so a later edit to the bank question cannot reach a student
// mid-test or change what a finished attempt was scored against (§7).
//
// media_asset_id points at the same immutable asset and the file is never
// copied -- an asset is content-addressed and already immutable (§11.1).
func snapshot(ctx context.Context, tx pgx.Tx, versionID string, d Draft) error {
	for _, section := range d.Sections {
		sectionID, err := freezeSection(ctx, tx, versionID, section)
		if err != nil {
			return err
		}
		for _, q := range section.Questions {
			if err := freezeQuestion(ctx, tx, sectionID, q); err != nil {
				return err
			}
		}
	}
	return nil
}

func freezeSection(ctx context.Context, tx pgx.Tx, versionID string, section Section) (string, error) {
	var id string
	if err := tx.QueryRow(ctx,
		`INSERT INTO app.test_version_sections (test_version_id, ordinal, title, instructions)
		 VALUES ($1, $2, $3, $4) RETURNING id::text`,
		versionID, section.Ordinal, section.Title, section.Instructions).Scan(&id); err != nil {
		return "", fmt.Errorf("publish: freeze section: %w", err)
	}
	return id, nil
}

func freezeQuestion(ctx context.Context, tx pgx.Tx, sectionID string, q Question) error {
	// The asset row is locked before the reference is frozen, so a concurrent
	// soft delete cannot slip between the delete's reference count and this
	// insert -- see media.LockForVersionUse.
	if q.MediaAssetID != nil {
		if err := media.LockForVersionUse(ctx, tx, *q.MediaAssetID); err != nil {
			return fmt.Errorf("publish: media asset %s: %w", *q.MediaAssetID, err)
		}
	}

	var id string
	if err := tx.QueryRow(ctx, `
		INSERT INTO app.test_version_questions
		       (test_version_section_id, ordinal, source_question_id, type, prompt,
		        media_asset_id, media_asset_kind, audio_max_plays, audio_allow_seek,
		        audio_show_transcript_after, transcript, points, explanation, sample_answer)
		VALUES ($1, $2, $3, $4::app.question_type, $5, $6, $7::app.media_kind, $8, $9, $10,
		        $11, $12::numeric, $13, $14)
		RETURNING id::text`,
		sectionID, q.Ordinal, q.SourceID, q.Type, q.Prompt,
		q.MediaAssetID, q.MediaAssetKind, q.MaxPlays, q.AllowSeek, q.ShowTranscript,
		q.Transcript, q.Points, q.Explanation, q.SampleAnswer).Scan(&id); err != nil {
		return fmt.Errorf("publish: freeze question: %w", err)
	}

	if err := freezeOptions(ctx, tx, id, q.Options); err != nil {
		return err
	}
	return freezeBlanks(ctx, tx, id, q.Blanks)
}

func freezeOptions(ctx context.Context, tx pgx.Tx, questionID string, options []Option) error {
	if len(options) == 0 {
		return nil
	}
	rows := make([][]any, len(options))
	for i, o := range options {
		rows[i] = []any{questionID, o.Ordinal, o.Text, o.IsCorrect}
	}
	if _, err := tx.CopyFrom(ctx,
		pgx.Identifier{"app", "test_version_options"},
		[]string{"test_version_question_id", "ordinal", "text", "is_correct"},
		pgx.CopyFromRows(rows)); err != nil {
		return fmt.Errorf("publish: freeze options: %w", err)
	}
	return nil
}

func freezeBlanks(ctx context.Context, tx pgx.Tx, questionID string, blanks []Blank) error {
	for _, b := range blanks {
		var blankID string
		if err := tx.QueryRow(ctx,
			`INSERT INTO app.test_version_blanks
			        (test_version_question_id, ordinal, case_sensitive)
			 VALUES ($1, $2, $3) RETURNING id::text`,
			questionID, b.Ordinal, b.CaseSensitive).Scan(&blankID); err != nil {
			return fmt.Errorf("publish: freeze blank: %w", err)
		}
		for _, answer := range b.AcceptedAnswers {
			if _, err := tx.Exec(ctx,
				`INSERT INTO app.test_version_blank_answers (test_version_blank_id, answer)
				 VALUES ($1, $2)`, blankID, answer); err != nil {
				return fmt.Errorf("publish: freeze blank answer: %w", err)
			}
		}
	}
	return nil
}
