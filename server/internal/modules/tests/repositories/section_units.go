package repositories

import (
	"context"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/db"
)

func readSectionUnits(ctx context.Context, q db.Querier, ids []string, byTest map[string][]domain.Section) error {
	rows, err := q.Query(ctx, `SELECT s.id::text,CASE WHEN u.group_id IS NULL THEN 'question' ELSE 'group' END,
  coalesce(u.group_id,u.question_id)::text FROM app.test_sections s
  JOIN app.test_section_units u ON u.test_section_id=s.id
  WHERE s.test_id=ANY($1::uuid[]) ORDER BY s.id,u.ordinal`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	units := map[string][]domain.SectionUnit{}
	for rows.Next() {
		var section string
		var unit domain.SectionUnit
		if err := rows.Scan(&section, &unit.Kind, &unit.ID); err != nil {
			return err
		}
		units[section] = append(units[section], unit)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, sections := range byTest {
		for i := range sections {
			sections[i].Units = units[sections[i].ID]
		}
	}
	return nil
}
