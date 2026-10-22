package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/platform/db"

	"github.com/jackc/pgx/v5"
)

// replaceMatchItems makes the question's matching items the ones sent, by the
// rule replaceOptions follows, keying each to optionIDs[CorrectOption]: the
// ids the question's options have once this save has written them. Items not
// sent are deleted first, so none of them names an option the save deletes.
func replaceMatchItems(ctx context.Context, tx pgx.Tx, questionID string, items []domain.MatchItemInput, optionIDs []string) error {
	held, err := heldChildren(ctx, tx, matchItemsTable, questionID)
	if err != nil {
		return fmt.Errorf("questions: read match items: %w", err)
	}
	kept := keptIDs(items, func(item domain.MatchItemInput) *string { return item.ID }, held)
	if err := deleteOtherChildren(ctx, tx, matchItemsTable, questionID, kept); err != nil {
		return fmt.Errorf("questions: clear match items: %w", err)
	}
	if len(items) == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `SET CONSTRAINTS app.question_match_items_ordinal_key DEFERRED`); err != nil {
		return fmt.Errorf("questions: defer match item order: %w", err)
	}
	batch := &pgx.Batch{}
	for i, item := range items {
		if item.CorrectOption < 0 || item.CorrectOption >= len(optionIDs) {
			return fmt.Errorf("questions: match item %d names option %d of %d", i, item.CorrectOption, len(optionIDs))
		}
		option := optionIDs[item.CorrectOption]
		if kept[i] == "" {
			batch.Queue(`INSERT INTO app.question_match_items (question_id, ordinal, text, correct_option_id)
				VALUES ($1, $2, $3, $4)`, questionID, i, item.Text, option)
			continue
		}
		batch.Queue(`UPDATE app.question_match_items SET ordinal = $2, text = $3, correct_option_id = $4
			WHERE id = $1 AND question_id = $5`, kept[i], i, item.Text, option, questionID)
	}
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("questions: write match items: %w", err)
	}
	if _, err := tx.Exec(ctx, `SET CONSTRAINTS app.question_match_items_ordinal_key IMMEDIATE`); err != nil {
		return fmt.Errorf("questions: order match items: %w", err)
	}
	return nil
}

// replaceRubric makes an essay's rubric the criteria sent, by the rule
// replaceOptions follows, and deletes every criterion of any other type.
func replaceRubric(ctx context.Context, tx pgx.Tx, questionID string, in domain.Input) error {
	criteria := in.Rubric
	if in.Type != domain.Essay {
		criteria = nil
	}
	held, err := heldChildren(ctx, tx, rubricTable, questionID)
	if err != nil {
		return fmt.Errorf("questions: read rubric: %w", err)
	}
	kept := keptIDs(criteria, func(c domain.RubricCriterionInput) *string { return c.ID }, held)
	if err := deleteOtherChildren(ctx, tx, rubricTable, questionID, kept); err != nil {
		return fmt.Errorf("questions: clear rubric: %w", err)
	}
	if len(criteria) == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `SET CONSTRAINTS app.question_rubric_criteria_ordinal_key DEFERRED`); err != nil {
		return fmt.Errorf("questions: defer rubric order: %w", err)
	}
	batch := &pgx.Batch{}
	for i, criterion := range criteria {
		if kept[i] == "" {
			batch.Queue(`INSERT INTO app.question_rubric_criteria (question_id, ordinal, label) VALUES ($1, $2, $3)`,
				questionID, i, criterion.Label)
			continue
		}
		batch.Queue(`UPDATE app.question_rubric_criteria SET ordinal = $2, label = $3 WHERE id = $1 AND question_id = $4`,
			kept[i], i, criterion.Label, questionID)
	}
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("questions: write rubric: %w", err)
	}
	if _, err := tx.Exec(ctx, `SET CONSTRAINTS app.question_rubric_criteria_ordinal_key IMMEDIATE`); err != nil {
		return fmt.Errorf("questions: order rubric: %w", err)
	}
	return nil
}

func (s *Postgres) loadMatchItemsFor(ctx context.Context, q db.Querier, questionIDs []string) (map[string][]domain.MatchItem, error) {
	if len(questionIDs) == 0 {
		return map[string][]domain.MatchItem{}, nil
	}
	byQuestion, err := db.GroupBy(ctx, q,
		`SELECT question_id::text, id::text, ordinal, text, correct_option_id::text
		   FROM app.question_match_items
		  WHERE question_id = ANY($1::uuid[])
		  ORDER BY question_id, ordinal`, []any{questionIDs},
		func(rows pgx.Rows) (string, domain.MatchItem, error) {
			var questionID string
			var item domain.MatchItem
			err := rows.Scan(&questionID, &item.ID, &item.Ordinal, &item.Text, &item.CorrectOptionID)
			return questionID, item, err
		})
	if err != nil {
		return nil, fmt.Errorf("questions: load match items: %w", err)
	}
	return byQuestion, nil
}

func (s *Postgres) loadRubricFor(ctx context.Context, q db.Querier, questionIDs []string) (map[string][]domain.RubricCriterion, error) {
	if len(questionIDs) == 0 {
		return map[string][]domain.RubricCriterion{}, nil
	}
	byQuestion, err := db.GroupBy(ctx, q,
		`SELECT question_id::text, id::text, ordinal, label
		   FROM app.question_rubric_criteria
		  WHERE question_id = ANY($1::uuid[])
		  ORDER BY question_id, ordinal`, []any{questionIDs},
		func(rows pgx.Rows) (string, domain.RubricCriterion, error) {
			var questionID string
			var criterion domain.RubricCriterion
			err := rows.Scan(&questionID, &criterion.ID, &criterion.Ordinal, &criterion.Label)
			return questionID, criterion, err
		})
	if err != nil {
		return nil, fmt.Errorf("questions: load rubric: %w", err)
	}
	return byQuestion, nil
}

// loadChildren reads every question's options, blanks, matching items and
// rubric in one query each, and skips the items or the rubric when no question
// of their type is there. Options and blanks are never nil; items and criteria
// are nil on a type that has none.
func (s *Postgres) loadChildren(ctx context.Context, q db.Querier, questions []domain.Question) error {
	if len(questions) == 0 {
		return nil
	}
	ids, matching, essays := childIDs(questions)
	options, err := s.loadOptionsFor(ctx, q, ids)
	if err != nil {
		return err
	}
	blanks, err := s.loadBlanksFor(ctx, q, ids)
	if err != nil {
		return err
	}
	items, err := s.loadMatchItemsFor(ctx, q, matching)
	if err != nil {
		return err
	}
	rubric, err := s.loadRubricFor(ctx, q, essays)
	if err != nil {
		return err
	}
	for i := range questions {
		question := &questions[i]
		question.Options = []domain.Option{}
		question.Blanks = []domain.Blank{}
		if got, ok := options[question.ID]; ok {
			question.Options = got
		}
		if got, ok := blanks[question.ID]; ok {
			question.Blanks = got
		}
		switch question.Type {
		case domain.Matching:
			question.MatchItems = keyedItems(items[question.ID], question.Options)
		case domain.Essay:
			question.Rubric = append([]domain.RubricCriterion{}, rubric[question.ID]...)
		}
	}
	return nil
}

// childIDs is every question's id, then those of the matching questions and
// those of the essays.
func childIDs(questions []domain.Question) (ids, matching, essays []string) {
	ids = make([]string, len(questions))
	for i, question := range questions {
		ids[i] = question.ID
		switch question.Type {
		case domain.Matching:
			matching = append(matching, question.ID)
		case domain.Essay:
			essays = append(essays, question.ID)
		}
	}
	return ids, matching, essays
}

// keyedItems is items with each one's CorrectOption set to the index of the
// option it names.
func keyedItems(items []domain.MatchItem, options []domain.Option) []domain.MatchItem {
	index := make(map[string]int, len(options))
	for i, option := range options {
		index[option.ID] = i
	}
	out := make([]domain.MatchItem, len(items))
	for i, item := range items {
		item.CorrectOption = index[item.CorrectOptionID]
		out[i] = item
	}
	return out
}
