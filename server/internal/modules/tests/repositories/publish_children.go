package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/db"

	"github.com/jackc/pgx/v5"
)

type typeChildTables struct {
	owner   string
	options string
	items   string
	rubric  string
}

const frozenOwner = "test_version_question_id"

var (
	bankChildren   = typeChildTables{owner: "question_id", options: "app.question_options", items: "app.question_match_items", rubric: "app.question_rubric_criteria"}
	frozenChildren = typeChildTables{owner: frozenOwner, options: "app.test_version_options", items: "app.test_version_match_items", rubric: "app.test_version_rubric_criteria"}
)

func loadMatchItems(ctx context.Context, tx pgx.Tx, tables typeChildTables, questionIDs []string) (map[string][]domain.DraftMatchItem, error) {
	if len(questionIDs) == 0 {
		return map[string][]domain.DraftMatchItem{}, nil
	}
	byQuestion, err := db.GroupBy(ctx, tx, `
		WITH positions AS (
		  SELECT `+tables.owner+` AS question_id, id, row_number() OVER (PARTITION BY `+tables.owner+` ORDER BY ordinal) - 1 AS position
		    FROM `+tables.options+` WHERE `+tables.owner+` = ANY($1::uuid[])
		)
		SELECT i.`+tables.owner+`::text, i.ordinal, i.text, coalesce(p.position, -1)::int
		  FROM `+tables.items+` i
		  LEFT JOIN positions p ON p.question_id = i.`+tables.owner+` AND p.id = i.correct_option_id
		 WHERE i.`+tables.owner+` = ANY($1::uuid[])
		 ORDER BY i.`+tables.owner+`, i.ordinal`, []any{questionIDs},
		func(rows pgx.Rows) (string, domain.DraftMatchItem, error) {
			var questionID string
			var item domain.DraftMatchItem
			err := rows.Scan(&questionID, &item.Ordinal, &item.Text, &item.CorrectOption)
			return questionID, item, err
		})
	if err != nil {
		return nil, fmt.Errorf("publish: load match items: %w", err)
	}
	return byQuestion, nil
}

func loadRubric(ctx context.Context, tx pgx.Tx, tables typeChildTables, questionIDs []string) (map[string][]domain.DraftCriterion, error) {
	if len(questionIDs) == 0 {
		return map[string][]domain.DraftCriterion{}, nil
	}
	byQuestion, err := db.GroupBy(ctx, tx, `SELECT `+tables.owner+`::text, ordinal, label FROM `+tables.rubric+`
		WHERE `+tables.owner+` = ANY($1::uuid[]) ORDER BY `+tables.owner+`, ordinal`, []any{questionIDs},
		func(rows pgx.Rows) (string, domain.DraftCriterion, error) {
			var questionID string
			var criterion domain.DraftCriterion
			err := rows.Scan(&questionID, &criterion.Ordinal, &criterion.Label)
			return questionID, criterion, err
		})
	if err != nil {
		return nil, fmt.Errorf("publish: load rubric: %w", err)
	}
	return byQuestion, nil
}

func attachTypeChildren(ctx context.Context, tx pgx.Tx, tables typeChildTables, bySection map[string][]domain.DraftQuestion, key func(domain.DraftQuestion) string) error {
	var ids []string
	for _, list := range bySection {
		for _, q := range list {
			ids = append(ids, key(q))
		}
	}
	items, err := loadMatchItems(ctx, tx, tables, ids)
	if err != nil {
		return err
	}
	rubric, err := loadRubric(ctx, tx, tables, ids)
	if err != nil {
		return err
	}
	for _, list := range bySection {
		for i := range list {
			list[i].MatchItems = items[key(list[i])]
			list[i].Rubric = rubric[key(list[i])]
		}
	}
	return nil
}
