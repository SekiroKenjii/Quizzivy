package repositories

import (
	"context"
	"quizzivy/internal/modules/tests/domain"
)

// SetArchived changes a bank group's archive state under its revision; test-owned groups follow their enclosing test lifecycle.
func (s *GroupsPostgres) SetArchived(ctx context.Context, in domain.GroupMutation, archived bool) (domain.StoredGroup, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.StoredGroup{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	stored, _, err := lockGroupMutation(ctx, tx, in)
	if err != nil {
		return domain.StoredGroup{}, err
	}
	if stored.OwnerSectionID != nil {
		return domain.StoredGroup{}, &domain.GroupError{Rule: "group_bank_only"}
	}
	if _, err := tx.Exec(ctx, `UPDATE app.question_groups SET archived_at=CASE WHEN $2 THEN $3::timestamptz ELSE NULL END,
		revision=revision+1 WHERE id=$1`, in.ID, archived, in.Now); err != nil {
		return domain.StoredGroup{}, err
	}
	action := "question_group.restored"
	if archived {
		action = "question_group.archived"
	}
	if err := recordGroupMutation(ctx, tx, in, "", action); err != nil {
		return domain.StoredGroup{}, err
	}
	return s.finishGroupMutation(ctx, tx, in.ID)
}

// Delete removes an archived bank group's entire independent graph, preserving append-only audit history.
func (s *GroupsPostgres) Delete(ctx context.Context, in domain.GroupMutation) error {
	tx, err := s.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	stored, _, err := lockGroupMutation(ctx, tx, in)
	if err != nil {
		return err
	}
	if stored.OwnerSectionID != nil {
		return &domain.GroupError{Rule: "group_bank_only"}
	}
	if stored.ArchivedAt == nil {
		return domain.ErrNotArchived
	}
	if _, err := lockGroupMembers(ctx, tx, in.ID); err != nil {
		return err
	}
	if err := clearGroupMaterials(ctx, tx, in.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM app.questions WHERE context_group_id=$1`, in.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM app.question_groups WHERE id=$1`, in.ID); err != nil {
		return err
	}
	if err := recordGroupMutation(ctx, tx, in, "", "question_group.deleted"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RemoveFromSection removes a test-owned draft group and compacts unit order under both draft and group revisions.
func (s *GroupsPostgres) RemoveFromSection(ctx context.Context, in domain.GroupMutation) error {
	tx, err := s.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	stored, testID, err := lockGroupMutation(ctx, tx, in)
	if err != nil {
		return err
	}
	if stored.OwnerSectionID == nil {
		return &domain.GroupError{Rule: "group_section_only"}
	}
	if _, err := lockGroupMembers(ctx, tx, in.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM app.test_section_units WHERE group_id=$1`, in.ID); err != nil {
		return err
	}
	if err := clearGroupMaterials(ctx, tx, in.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM app.questions WHERE context_group_id=$1`, in.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM app.question_groups WHERE id=$1`, in.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SET CONSTRAINTS app.test_section_units_ordinal_key DEFERRED`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `WITH ordered AS (
		SELECT id,row_number() OVER (ORDER BY ordinal)-1 AS ordinal FROM app.test_section_units WHERE test_section_id=$1
	) UPDATE app.test_section_units u SET ordinal=o.ordinal FROM ordered o WHERE u.id=o.id`, *stored.OwnerSectionID); err != nil {
		return err
	}
	if err := recordGroupMutation(ctx, tx, in, testID, "question_group.removed"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
