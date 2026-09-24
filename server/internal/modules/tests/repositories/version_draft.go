package repositories

import (
	"context"
	"quizzivy/internal/modules/tests/domain"

	"github.com/jackc/pgx/v5"
)

type snapshotSection struct {
	ID           string
	Title        string
	Instructions *string
}

func (s *Postgres) copyVersionDraft(ctx context.Context, tx pgx.Tx, versionID, actorID string) ([]domain.SectionInput, error) {
	rows, err := tx.Query(ctx, `SELECT id::text, title, instructions FROM app.test_version_sections WHERE test_version_id = $1 ORDER BY ordinal`, versionID)
	if err != nil {
		return nil, err
	}
	sections, err := pgx.CollectRows(rows, pgx.RowToStructByPos[snapshotSection])
	if err != nil {
		return nil, err
	}
	out := make([]domain.SectionInput, 0, len(sections))
	for _, section := range sections {
		ids, err := s.copySectionQuestions(ctx, tx, section.ID, actorID)
		if err != nil {
			return nil, err
		}
		out = append(out, domain.SectionInput{Title: section.Title, Instructions: section.Instructions, QuestionIDs: ids})
	}
	return out, nil
}

func (s *Postgres) copySectionQuestions(ctx context.Context, tx pgx.Tx, sectionID, actorID string) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT id::text FROM app.test_version_questions WHERE test_version_section_id = $1 ORDER BY ordinal`, sectionID)
	if err != nil {
		return nil, err
	}
	sources, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(sources))
	for _, source := range sources {
		id, err := copySnapshotQuestion(ctx, tx, source, actorID)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func copySnapshotQuestion(ctx context.Context, tx pgx.Tx, sourceID, actorID string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `INSERT INTO app.questions
  (type, prompt, media_asset_id, media_asset_kind, audio_max_plays, audio_allow_seek,
   audio_show_transcript_after, transcript, points, explanation, sample_answer, created_by, prompt_content, explanation_content)
  SELECT type, prompt, media_asset_id, media_asset_kind, audio_max_plays, audio_allow_seek,
   audio_show_transcript_after, transcript, points, explanation, sample_answer, $2, prompt_content, explanation_content
  FROM app.test_version_questions WHERE id = $1 RETURNING id::text`, sourceID, actorID).Scan(&id)
	if err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.question_options (question_id, ordinal, text, is_correct, content)
  SELECT $2, ordinal, text, is_correct, content FROM app.test_version_options WHERE test_version_question_id = $1`, sourceID, id); err != nil {
		return "", err
	}
	if err := copySnapshotBlanks(ctx, tx, sourceID, id); err != nil {
		return "", err
	}
	return id, nil
}

func copySnapshotBlanks(ctx context.Context, tx pgx.Tx, sourceID, questionID string) error {
	_, err := tx.Exec(ctx, `WITH copied AS (
  INSERT INTO app.question_blanks (question_id, ordinal, case_sensitive)
  SELECT $2, ordinal, case_sensitive FROM app.test_version_blanks WHERE test_version_question_id = $1
  RETURNING id, ordinal
 )
 INSERT INTO app.question_blank_answers (blank_id, answer)
 SELECT copied.id, a.answer FROM copied
 JOIN app.test_version_blanks b ON b.test_version_question_id = $1 AND b.ordinal = copied.ordinal
 JOIN app.test_version_blank_answers a ON a.test_version_blank_id = b.id`, sourceID, questionID)
	return err
}
