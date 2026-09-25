package repositories

import (
	"context"
	"encoding/json"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/content"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type snapshotSection struct {
	ID           string
	Title        string
	Instructions *string
}

type copiedSnapshotSection struct {
	SourceID  string
	Input     domain.SectionInput
	Questions map[string]string
}

func (s *Postgres) copyVersionDraft(ctx context.Context, tx pgx.Tx, versionID, actorID string) ([]copiedSnapshotSection, error) {
	rows, err := tx.Query(ctx, `SELECT id::text, title, instructions FROM app.test_version_sections WHERE test_version_id = $1 ORDER BY ordinal`, versionID)
	if err != nil {
		return nil, err
	}
	sections, err := pgx.CollectRows(rows, pgx.RowToStructByPos[snapshotSection])
	if err != nil {
		return nil, err
	}
	out := make([]copiedSnapshotSection, 0, len(sections))
	for _, section := range sections {
		ids, bySource, err := copySectionQuestions(ctx, tx, section.ID, actorID)
		if err != nil {
			return nil, err
		}
		out = append(out, copiedSnapshotSection{SourceID: section.ID, Input: domain.SectionInput{Title: section.Title, Instructions: section.Instructions, QuestionIDs: ids}, Questions: bySource})
	}
	return out, nil
}

func copySectionQuestions(ctx context.Context, tx pgx.Tx, sectionID, actorID string) ([]string, map[string]string, error) {
	rows, err := tx.Query(ctx, `SELECT q.id::text FROM app.test_version_questions q WHERE q.test_version_section_id=$1
		AND NOT EXISTS (SELECT 1 FROM app.test_version_group_members m WHERE m.question_id=q.id) ORDER BY q.ordinal`, sectionID)
	if err != nil {
		return nil, nil, err
	}
	sources, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, nil, err
	}
	ids := make([]string, 0, len(sources))
	bySource := make(map[string]string, len(sources))
	for _, source := range sources {
		id, err := copySnapshotQuestion(ctx, tx, source, actorID)
		if err != nil {
			return nil, nil, err
		}
		ids = append(ids, id)
		bySource[source] = id
	}
	return ids, bySource, nil
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
	if err := rebindCopiedGaps(ctx, tx, id); err != nil {
		return "", err
	}
	return id, nil
}

func rebindCopiedGaps(ctx context.Context, tx pgx.Tx, questionID string) error {
	var raw json.RawMessage
	if err := tx.QueryRow(ctx, `SELECT prompt_content FROM app.questions WHERE id = $1`, questionID).Scan(&raw); err != nil {
		return err
	}
	if len(raw) == 0 {
		return nil
	}
	document, err := content.ParseQuestionPrompt(raw)
	if err != nil || len(document.GapIDs()) == 0 {
		return err
	}
	oldIDs := document.GapIDs()
	newIDs := make([]string, len(oldIDs))
	ids := make(map[string]string, len(oldIDs))
	for i, id := range oldIDs {
		newIDs[i] = uuid.NewString()
		ids[id] = newIDs[i]
	}
	copied, err := document.WithGapIDs(ids)
	if err != nil {
		return err
	}
	encoded, err := copied.MarshalJSON()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE app.question_blanks b SET gap_id = m.new_id
      FROM unnest($2::text[], $3::text[]) AS m(old_id, new_id)
      WHERE b.question_id = $1 AND b.gap_id = m.old_id`, questionID, oldIDs, newIDs); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE app.questions SET prompt_content = $2 WHERE id = $1`, questionID, encoded)
	return err
}

func copySnapshotBlanks(ctx context.Context, tx pgx.Tx, sourceID, questionID string) error {
	_, err := tx.Exec(ctx, `WITH copied AS (
  INSERT INTO app.question_blanks (question_id, ordinal, case_sensitive, gap_id)
  SELECT $2, ordinal, case_sensitive, gap_id FROM app.test_version_blanks WHERE test_version_question_id = $1
  RETURNING id, ordinal
 )
 INSERT INTO app.question_blank_answers (blank_id, answer)
 SELECT copied.id, a.answer FROM copied
 JOIN app.test_version_blanks b ON b.test_version_question_id = $1 AND b.ordinal = copied.ordinal
 JOIN app.test_version_blank_answers a ON a.test_version_blank_id = b.id`, sourceID, questionID)
	return err
}
