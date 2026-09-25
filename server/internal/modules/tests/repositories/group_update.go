package repositories

import (
	"context"
	"fmt"
	questions "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/tests/domain"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Update replaces a complete group under its aggregate revision and parent lock; stale writes and invalid graphs leave no partial changes.
func (s *GroupsPostgres) Update(ctx context.Context, in domain.UpdateGroupInput) (domain.StoredGroup, error) {
	stored, err := s.update(ctx, in)
	return stored, groupWriteError(err)
}

func (s *GroupsPostgres) update(ctx context.Context, in domain.UpdateGroupInput) (domain.StoredGroup, error) {
	if s.questions == nil {
		return domain.StoredGroup{}, fmt.Errorf("groups: question store unavailable")
	}
	if !strings.EqualFold(in.ID, in.Bundle.Group.ID) {
		return domain.StoredGroup{}, &domain.GroupError{Rule: "group_identity"}
	}
	if err := in.Bundle.Validate(); err != nil {
		return domain.StoredGroup{}, err
	}
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.StoredGroup{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	stored, testID, err := lockGroupMutation(ctx, tx, in.GroupMutation)
	if err != nil {
		return domain.StoredGroup{}, err
	}
	if stored.ArchivedAt != nil {
		return domain.StoredGroup{}, &domain.GroupError{Rule: "group_archived"}
	}
	if err := s.replaceGroupGraph(ctx, tx, in); err != nil {
		return domain.StoredGroup{}, err
	}
	if err := recordGroupMutation(ctx, tx, in.GroupMutation, testID, "question_group.updated"); err != nil {
		return domain.StoredGroup{}, err
	}
	return s.finishGroupMutation(ctx, tx, in.ID)
}

func (s *GroupsPostgres) replaceGroupGraph(ctx context.Context, tx pgx.Tx, in domain.UpdateGroupInput) error {
	ids, err := lockGroupMembers(ctx, tx, in.ID)
	if err != nil {
		return err
	}
	if err := s.lockGroupAssets(ctx, tx, in.Bundle); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SET CONSTRAINTS app.questions_context_ordinal_key DEFERRED`); err != nil {
		return err
	}
	if err := clearGroupMaterials(ctx, tx, in.ID); err != nil {
		return err
	}
	if err := s.replaceGroupMembers(ctx, tx, in, ids); err != nil {
		return err
	}
	if err := insertGroupRecordings(ctx, tx, in.Bundle.Group); err != nil {
		return err
	}
	if err := insertGroupMaterials(ctx, tx, in.Bundle.Group); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE app.question_groups SET title=$2,instructions=$3,revision=revision+1 WHERE id=$1`, in.ID, in.Bundle.Group.Title, nullableGroupContent(in.Bundle.Group.Instructions))
	return err
}

func (s *GroupsPostgres) replaceGroupMembers(ctx context.Context, tx pgx.Tx, in domain.UpdateGroupInput, previous []string) error {
	existing := make(map[string]bool, len(previous))
	for _, id := range previous {
		existing[id] = true
	}
	catalog := make(map[string]domain.GroupQuestion, len(in.Bundle.Questions))
	retained := make([]string, 0, len(in.Bundle.Questions))
	for _, question := range in.Bundle.Questions {
		id := strings.ToLower(question.ID)
		catalog[id] = question
		retained = append(retained, id)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM app.questions WHERE context_group_id=$1 AND NOT (id=ANY($2::uuid[]))`, in.ID, retained); err != nil {
		return err
	}
	for i, member := range in.Bundle.Group.Members {
		id := strings.ToLower(member.QuestionID)
		question := catalog[id]
		write := questions.WriteInput{ID: question.ID, Input: question.Input, MediaAssetKind: question.MediaAssetKind,
			ActorID: in.ActorID, Now: in.Now, IP: in.IP, UserAgent: in.UserAgent}
		owner := questions.GroupOwnership{GroupID: in.ID, Ordinal: i, OptionOrder: member.OptionOrder}
		var err error
		if existing[id] {
			_, err = s.questions.UpdateGroupMember(ctx, tx, write, owner)
		} else {
			_, err = s.questions.CreateGroupMember(ctx, tx, write, owner)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *GroupsPostgres) finishGroupMutation(ctx context.Context, tx pgx.Tx, id string) (domain.StoredGroup, error) {
	stored, err := readGroup(ctx, tx, id)
	if err != nil {
		return domain.StoredGroup{}, err
	}
	if err := s.readGraph(ctx, tx, &stored); err != nil {
		return domain.StoredGroup{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.StoredGroup{}, err
	}
	return stored, nil
}
