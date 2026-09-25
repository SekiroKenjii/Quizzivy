package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/db"

	"github.com/jackc/pgx/v5"
)

// Preview renders a published version the way a student would receive it.
func (s *Postgres) Preview(ctx context.Context, testID string, version int) (domain.PreviewPaper, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.PreviewPaper{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var versionID string
	var paper domain.PreviewPaper
	err = tx.QueryRow(ctx, `
        SELECT v.id::text, v.version FROM app.test_versions v
        JOIN app.tests t ON t.id=v.test_id
        WHERE v.test_id=$1 AND t.deleted_at IS NULL
        AND v.version=CASE WHEN $2::integer=0 THEN t.current_version ELSE $2 END
        FOR SHARE OF v`, testID, version).Scan(&versionID, &paper.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PreviewPaper{}, domain.ErrNotPublished
	}
	if err != nil {
		return domain.PreviewPaper{}, err
	}
	bound := NewPostgres(db.NewContext(tx), s.questions, s.media)
	paper.Questions, err = bound.previewQuestions(ctx, versionID)
	if err != nil {
		return domain.PreviewPaper{}, err
	}
	paper.Sections, err = db.QueryMany(ctx, tx, `SELECT id::text,title,instructions
        FROM app.test_version_sections WHERE test_version_id=$1 ORDER BY ordinal`, []any{versionID},
		func(rows pgx.Rows) (domain.PreviewSection, error) {
			var section domain.PreviewSection
			err := rows.Scan(&section.ID, &section.Title, &section.Instructions)
			return section, err
		})
	if err != nil {
		return domain.PreviewPaper{}, err
	}
	paper.Groups, err = previewGroups(ctx, tx, versionID)
	if err != nil {
		return domain.PreviewPaper{}, err
	}
	return paper, tx.Commit(ctx)
}

func (s *Postgres) previewQuestions(ctx context.Context, versionID string) ([]domain.PreviewQuestion, error) {
	rows, err := s.Query(ctx, `
		SELECT q.id::text, q.test_version_section_id::text, q.type::text, q.prompt, q.prompt_content, q.points::text,
		       q.media_asset_id::text, q.audio_max_plays, q.audio_allow_seek,
		       q.audio_show_transcript_after
		  FROM app.test_version_sections s
		  JOIN app.test_version_questions q ON q.test_version_section_id = s.id
		 WHERE s.test_version_id = $1
		 ORDER BY s.ordinal, q.ordinal`, versionID)
	if err != nil {
		return nil, fmt.Errorf("tests: preview questions: %w", err)
	}
	defer rows.Close()

	var out []domain.PreviewQuestion
	byID := map[string]int{}
	for rows.Next() {
		var q domain.PreviewQuestion
		if err := rows.Scan(&q.ID, &q.SectionID, &q.Type, &q.Prompt, &q.PromptContent, &q.Points, &q.MediaAssetID,
			&q.MaxPlays, &q.AllowSeek, &q.ShowScript); err != nil {
			return nil, fmt.Errorf("tests: scan preview question: %w", err)
		}
		byID[q.ID] = len(out)
		out = append(out, q)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("tests: preview questions: %w", err)
	}
	if len(out) == 0 {
		return out, nil
	}

	if err := s.attachPreviewOptions(ctx, versionID, out, byID); err != nil {
		return nil, err
	}
	return out, s.attachPreviewBlanks(ctx, versionID, out, byID)
}

func (s *Postgres) attachPreviewOptions(
	ctx context.Context, versionID string, out []domain.PreviewQuestion, byID map[string]int,
) error {
	byQuestion, err := db.GroupBy(ctx, s.Conn(), `
		SELECT o.test_version_question_id::text, o.id::text, o.text, o.content
		  FROM app.test_version_sections s
		  JOIN app.test_version_questions q ON q.test_version_section_id = s.id
		  JOIN app.test_version_options o ON o.test_version_question_id = q.id
		 WHERE s.test_version_id = $1
		 ORDER BY o.ordinal`, []any{versionID},
		func(rows pgx.Rows) (string, domain.PreviewOption, error) {
			var questionID string
			var option domain.PreviewOption
			err := rows.Scan(&questionID, &option.ID, &option.Text, &option.Content)
			return questionID, option, err
		})
	if err != nil {
		return fmt.Errorf("tests: preview options: %w", err)
	}
	for questionID, options := range byQuestion {
		if i, ok := byID[questionID]; ok {
			out[i].Options = options
		}
	}
	return nil
}

func (s *Postgres) attachPreviewBlanks(
	ctx context.Context, versionID string, out []domain.PreviewQuestion, byID map[string]int,
) error {
	byQuestion, err := db.GroupBy(ctx, s.Conn(), `
		SELECT b.test_version_question_id::text, b.id::text, b.ordinal, b.gap_id, b.case_sensitive
		  FROM app.test_version_sections s
		  JOIN app.test_version_questions q ON q.test_version_section_id = s.id
		  JOIN app.test_version_blanks b ON b.test_version_question_id = q.id
		 WHERE s.test_version_id = $1
		 ORDER BY b.ordinal`, []any{versionID},
		func(rows pgx.Rows) (string, domain.PreviewBlank, error) {
			var questionID string
			var blank domain.PreviewBlank
			err := rows.Scan(&questionID, &blank.ID, &blank.Ordinal, &blank.GapID, &blank.CaseSensitive)
			return questionID, blank, err
		})
	if err != nil {
		return fmt.Errorf("tests: preview blanks: %w", err)
	}
	for questionID, blanks := range byQuestion {
		if i, ok := byID[questionID]; ok {
			out[i].Blanks = blanks
		}
	}
	return nil
}
