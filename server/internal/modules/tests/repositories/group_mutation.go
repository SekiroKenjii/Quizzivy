package repositories

import (
	"context"
	"errors"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"

	"github.com/jackc/pgx/v5"
)

func lockGroupMutation(ctx context.Context, tx pgx.Tx, in domain.GroupMutation) (domain.StoredGroup, string, error) {
	var testID *string
	err := tx.QueryRow(ctx, `SELECT s.test_id::text FROM app.question_groups g
		LEFT JOIN app.test_sections s ON s.id=g.owner_section_id WHERE g.id=$1`, in.ID).Scan(&testID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.StoredGroup{}, "", domain.ErrNotFound
	}
	if err != nil {
		return domain.StoredGroup{}, "", err
	}
	if testID != nil {
		if err := checkVersion(ctx, tx, *testID, in.ExpectedTestUpdatedAt); err != nil {
			return domain.StoredGroup{}, "", err
		}
		var archived bool
		if err := tx.QueryRow(ctx, `SELECT status='archived' FROM app.tests WHERE id=$1`, *testID).Scan(&archived); err != nil {
			return domain.StoredGroup{}, "", err
		}
		if archived {
			return domain.StoredGroup{}, "", &domain.GroupError{Rule: "group_owner_archived"}
		}
	}
	stored, err := readLockedGroup(ctx, tx, in.ID, "FOR UPDATE")
	if err != nil {
		return domain.StoredGroup{}, "", err
	}
	if stored.Revision != in.ExpectedRevision {
		return domain.StoredGroup{}, "", domain.ErrStaleWrite
	}
	if testID == nil {
		return stored, "", nil
	}
	return stored, *testID, nil
}

func lockGroupMembers(ctx context.Context, tx pgx.Tx, id string) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT id::text FROM app.questions WHERE context_group_id=$1 ORDER BY id FOR UPDATE`, id)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func recordGroupMutation(ctx context.Context, tx pgx.Tx, in domain.GroupMutation, testID, action string) error {
	if testID != "" {
		if _, err := tx.Exec(ctx, `UPDATE app.tests SET updated_at=$2 WHERE id=$1`, testID, in.Now); err != nil {
			return err
		}
	}
	return audit.Write(ctx, tx, audit.Entry{ActorUserID: &in.ActorID, Action: action, Entity: "question_group", EntityID: &in.ID, OccurredAt: in.Now, IP: opt.String(in.IP), UserAgent: opt.String(in.UserAgent)})
}

func clearGroupMaterials(ctx context.Context, tx pgx.Tx, id string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM app.group_stimuli WHERE group_id=$1`, id); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `DELETE FROM app.group_recordings WHERE group_id=$1`, id)
	return err
}
