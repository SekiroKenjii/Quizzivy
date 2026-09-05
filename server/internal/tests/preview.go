package tests

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"quizzivy/internal/db"
)

// ErrNotPublished is returned when a test has no version to render.
var ErrNotPublished = errors.New("tests: no published version")

// PreviewQuestion is one question as a student receives it.
//
// It carries no is_correct, no accepted answer, no sample answer and no
// transcript. That is not a projection applied on the way out -- those columns
// are never selected, so nothing downstream can leak one by forgetting to strip
// it (§14 E2E 9).
type PreviewQuestion struct {
	ID           string
	Type         string
	Prompt       string
	Points       string
	MediaAssetID *string
	MaxPlays     *int
	AllowSeek    *bool
	ShowScript   *bool
	Options      []PreviewOption
	Blanks       []PreviewBlank
}

type PreviewOption struct {
	ID   string
	Text string
}

type PreviewBlank struct {
	ID      string
	Ordinal int
}

// Preview renders a published version the way a student would receive it.
//
// version 0 means the test's current one. A draft that has never been published
// has nothing to render, which is ErrNotPublished rather than an empty list: an
// empty preview would look like a published test with no questions.
func (s *Store) Preview(ctx context.Context, testID string, version int) (int, []PreviewQuestion, error) {
	var versionID string
	var resolved int
	err := s.pool.QueryRow(ctx, `
		SELECT v.id::text, v.version
		  FROM app.test_versions v
		 WHERE v.test_id = $1
		   AND ($2 = 0 OR v.version = $2)
		 ORDER BY v.version DESC
		 LIMIT 1`, testID, version).Scan(&versionID, &resolved)
	if err != nil {
		return 0, nil, ErrNotPublished
	}

	questions, err := s.previewQuestions(ctx, versionID)
	if err != nil {
		return 0, nil, err
	}
	return resolved, questions, nil
}

func (s *Store) previewQuestions(ctx context.Context, versionID string) ([]PreviewQuestion, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT q.id::text, q.type::text, q.prompt, q.points::text,
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

	var out []PreviewQuestion
	byID := map[string]int{}
	for rows.Next() {
		var q PreviewQuestion
		if err := rows.Scan(&q.ID, &q.Type, &q.Prompt, &q.Points, &q.MediaAssetID,
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

// Two queries for the children regardless of page size, the same shape the
// question bank uses: one per question would be a round trip per row.
func (s *Store) attachPreviewOptions(
	ctx context.Context, versionID string, out []PreviewQuestion, byID map[string]int,
) error {
	byQuestion, err := db.GroupBy(ctx, s.pool, `
		SELECT o.test_version_question_id::text, o.id::text, o.text
		  FROM app.test_version_sections s
		  JOIN app.test_version_questions q ON q.test_version_section_id = s.id
		  JOIN app.test_version_options o ON o.test_version_question_id = q.id
		 WHERE s.test_version_id = $1
		 ORDER BY o.ordinal`, []any{versionID},
		func(rows pgx.Rows) (string, PreviewOption, error) {
			var questionID string
			var option PreviewOption
			err := rows.Scan(&questionID, &option.ID, &option.Text)
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

func (s *Store) attachPreviewBlanks(
	ctx context.Context, versionID string, out []PreviewQuestion, byID map[string]int,
) error {
	byQuestion, err := db.GroupBy(ctx, s.pool, `
		SELECT b.test_version_question_id::text, b.id::text, b.ordinal
		  FROM app.test_version_sections s
		  JOIN app.test_version_questions q ON q.test_version_section_id = s.id
		  JOIN app.test_version_blanks b ON b.test_version_question_id = q.id
		 WHERE s.test_version_id = $1
		 ORDER BY b.ordinal`, []any{versionID},
		func(rows pgx.Rows) (string, PreviewBlank, error) {
			var questionID string
			var blank PreviewBlank
			err := rows.Scan(&questionID, &blank.ID, &blank.Ordinal)
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
