package repositories

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func clearTestGroups(ctx context.Context, tx pgx.Tx, testID string) error {
	rows, err := tx.Query(ctx, `SELECT g.id::text FROM app.question_groups g JOIN app.test_sections s ON s.id=g.owner_section_id
		WHERE s.test_id=$1 ORDER BY g.id FOR UPDATE OF g`, testID)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := lockGroupMembers(ctx, tx, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM app.test_section_units WHERE group_id=$1`, id); err != nil {
			return err
		}
		if err := clearGroupMaterials(ctx, tx, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM app.questions WHERE context_group_id=$1`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM app.question_groups WHERE id=$1`, id); err != nil {
			return err
		}
	}
	return nil
}
