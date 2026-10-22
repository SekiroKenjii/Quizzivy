package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/matchkey"

	"github.com/jackc/pgx/v5"
)

func (s *Reviews) attachMatchItems(ctx context.Context, versionID string, qs []domain.ReviewQuestion, at map[string]int) error {
	byQuestion, err := db.GroupBy(ctx, s.Conn(), `
		SELECT i.test_version_question_id::text, i.id::text, i.ordinal, i.text, i.correct_option_id::text
		  FROM app.test_version_match_items i
		  JOIN app.test_version_questions q ON q.id = i.test_version_question_id
		  JOIN app.test_version_sections s ON s.id = q.test_version_section_id
		 WHERE s.test_version_id = $1::uuid
		 ORDER BY i.ordinal`, []any{versionID},
		func(rows pgx.Rows) (string, domain.ReviewMatchItem, error) {
			var questionID string
			var it domain.ReviewMatchItem
			err := rows.Scan(&questionID, &it.ID, &it.Ordinal, &it.Text, &it.CorrectOptionID)
			it.Key = matchkey.Item(it.Ordinal)
			return questionID, it, err
		})
	if err != nil {
		return fmt.Errorf("review: read match items: %w", err)
	}
	for questionID, items := range byQuestion {
		if i, ok := at[questionID]; ok {
			qs[i].MatchItems = items
		}
	}
	return nil
}

func (s *Reviews) attachRubric(ctx context.Context, versionID string, qs []domain.ReviewQuestion, at map[string]int) error {
	byQuestion, err := db.GroupBy(ctx, s.Conn(), `
		SELECT c.test_version_question_id::text, c.id::text, c.ordinal, c.label
		  FROM app.test_version_rubric_criteria c
		  JOIN app.test_version_questions q ON q.id = c.test_version_question_id
		  JOIN app.test_version_sections s ON s.id = q.test_version_section_id
		 WHERE s.test_version_id = $1::uuid
		 ORDER BY c.ordinal`, []any{versionID},
		func(rows pgx.Rows) (string, domain.RubricCriterion, error) {
			var questionID string
			var c domain.RubricCriterion
			err := rows.Scan(&questionID, &c.ID, &c.Ordinal, &c.Label)
			return questionID, c, err
		})
	if err != nil {
		return fmt.Errorf("review: read rubric: %w", err)
	}
	for questionID, rubric := range byQuestion {
		if i, ok := at[questionID]; ok {
			qs[i].Rubric = rubric
		}
	}
	return nil
}

func (s *Reviews) givenCriteria(ctx context.Context, where string, args ...any) (map[string][]domain.CriterionBand, error) {
	given, err := db.GroupBy(ctx, s.Conn(), `
		SELECT g.attempt_id::text || ':' || g.question_id::text, g.criterion_id::text, g.band
		  FROM app.attempt_answer_criteria g
		  JOIN app.test_version_rubric_criteria c ON c.id = g.criterion_id
		 WHERE `+where+`
		 ORDER BY c.ordinal`, args,
		func(rows pgx.Rows) (string, domain.CriterionBand, error) {
			var key string
			var c domain.CriterionBand
			err := rows.Scan(&key, &c.CriterionID, &c.Band)
			return key, c, err
		})
	if err != nil {
		return nil, fmt.Errorf("review: read given criteria: %w", err)
	}
	return given, nil
}
