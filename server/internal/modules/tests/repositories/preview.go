package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/db"

	"github.com/jackc/pgx/v5"
)

// Preview renders a published version the way a student would receive it.
func (s *Postgres) Preview(ctx context.Context, testID string, version int) (int, []domain.PreviewQuestion, error) {
	var versionID string
	var resolved int
	err := s.QueryRow(ctx, `
		SELECT v.id::text, v.version
		  FROM app.test_versions v
		 WHERE v.test_id = $1
		   AND ($2 = 0 OR v.version = $2)
		 ORDER BY v.version DESC
		 LIMIT 1`, testID, version).Scan(&versionID, &resolved)
	if err != nil {
		return 0, nil, domain.ErrNotPublished
	}

	questions, err := s.previewQuestions(ctx, versionID)
	if err != nil {
		return 0, nil, err
	}
	return resolved, questions, nil
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
