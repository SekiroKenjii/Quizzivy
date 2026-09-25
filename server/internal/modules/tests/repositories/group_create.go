package repositories

import (
	"context"
	"errors"
	"fmt"
	questions "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Create writes the complete independent graph and its audit entry in one transaction; no partial group survives failure.
func (s *GroupsPostgres) Create(ctx context.Context, in domain.CreateGroupInput) (domain.StoredGroup, error) {
	stored, err := s.create(ctx, in)
	return stored, groupWriteError(err)
}

func (s *GroupsPostgres) create(ctx context.Context, in domain.CreateGroupInput) (domain.StoredGroup, error) {
	if err := in.Bundle.Validate(); err != nil {
		return domain.StoredGroup{}, err
	}
	if s.questions == nil {
		return domain.StoredGroup{}, fmt.Errorf("groups: question store unavailable")
	}
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.StoredGroup{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	testID, err := lockGroupOwner(ctx, tx, in)
	if err != nil {
		return domain.StoredGroup{}, err
	}
	if err := insertGroup(ctx, tx, in); err != nil {
		return domain.StoredGroup{}, err
	}
	if err := s.mountGroup(ctx, tx, in); err != nil {
		return domain.StoredGroup{}, err
	}
	if err := s.lockGroupAssets(ctx, tx, in.Bundle); err != nil {
		return domain.StoredGroup{}, err
	}
	if err := s.insertGroupMembers(ctx, tx, in); err != nil {
		return domain.StoredGroup{}, err
	}
	if err := insertGroupRecordings(ctx, tx, in.Bundle.Group); err != nil {
		return domain.StoredGroup{}, err
	}
	if err := insertGroupMaterials(ctx, tx, in.Bundle.Group); err != nil {
		return domain.StoredGroup{}, err
	}
	if err := recordGroupCreation(ctx, tx, in, testID); err != nil {
		return domain.StoredGroup{}, err
	}
	stored, err := readGroup(ctx, tx, in.Bundle.Group.ID)
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

func recordGroupCreation(ctx context.Context, tx pgx.Tx, in domain.CreateGroupInput, testID string) error {
	if testID != "" {
		if _, err := tx.Exec(ctx, `UPDATE app.tests SET updated_at=$2 WHERE id=$1`, testID, in.Now); err != nil {
			return err
		}
	}
	return audit.Write(ctx, tx, audit.Entry{ActorUserID: &in.ActorID, Action: "question_group.created", Entity: "question_group", EntityID: &in.Bundle.Group.ID, OccurredAt: in.Now, IP: opt.String(in.IP), UserAgent: opt.String(in.UserAgent)})
}

func lockGroupOwner(ctx context.Context, tx pgx.Tx, in domain.CreateGroupInput) (string, error) {
	if in.OwnerSectionID == nil {
		return "", nil
	}
	var testID string
	err := tx.QueryRow(ctx, `SELECT test_id::text FROM app.test_sections WHERE id=$1`, *in.OwnerSectionID).Scan(&testID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if err := checkVersion(ctx, tx, testID, in.ExpectedTestUpdatedAt); err != nil {
		return "", err
	}
	var archived bool
	if err := tx.QueryRow(ctx, `SELECT status='archived' FROM app.tests WHERE id=$1`, testID).Scan(&archived); err != nil {
		return "", err
	}
	if archived {
		return "", &domain.GroupError{Rule: "group_owner_archived"}
	}
	return testID, nil
}

func insertGroup(ctx context.Context, tx pgx.Tx, in domain.CreateGroupInput) error {
	_, err := tx.Exec(ctx, `INSERT INTO app.question_groups (id,owner_section_id,title,instructions,created_by)
		VALUES ($1,$2,$3,$4,$5)`, in.Bundle.Group.ID, in.OwnerSectionID, in.Bundle.Group.Title, nullableGroupContent(in.Bundle.Group.Instructions), in.ActorID)
	return err
}

func (s *GroupsPostgres) insertGroupMembers(ctx context.Context, tx pgx.Tx, in domain.CreateGroupInput) error {
	catalog := make(map[string]domain.GroupQuestion)
	for _, question := range in.Bundle.Questions {
		catalog[strings.ToLower(question.ID)] = question
	}
	for i, member := range in.Bundle.Group.Members {
		question := catalog[strings.ToLower(member.QuestionID)]
		_, err := s.questions.CreateGroupMember(ctx, tx, questions.WriteInput{
			ID: question.ID, Input: question.Input, MediaAssetKind: question.MediaAssetKind,
			ActorID: in.ActorID, Now: in.Now, IP: in.IP, UserAgent: in.UserAgent,
		}, questions.GroupOwnership{GroupID: in.Bundle.Group.ID, Ordinal: i, OptionOrder: member.OptionOrder})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *GroupsPostgres) mountGroup(ctx context.Context, tx pgx.Tx, in domain.CreateGroupInput) error {
	if in.OwnerSectionID == nil {
		return nil
	}
	sectionID := *in.OwnerSectionID
	var populated bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM app.test_section_units WHERE test_section_id=$1)`, sectionID).Scan(&populated); err != nil {
		return err
	}
	if !populated {
		if err := s.seedSectionUnits(ctx, tx, sectionID); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `INSERT INTO app.test_section_units (test_section_id,ordinal,group_id)
		SELECT $1,coalesce(max(ordinal)+1,0),$2 FROM app.test_section_units WHERE test_section_id=$1`, sectionID, in.Bundle.Group.ID)
	return err
}

func (s *GroupsPostgres) seedSectionUnits(ctx context.Context, tx pgx.Tx, sectionID string) error {
	rows, err := tx.Query(ctx, `SELECT question_id::text FROM app.test_section_questions WHERE test_section_id=$1 ORDER BY question_id`, sectionID)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.questions.LockForDraftUse(ctx, tx, id); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO app.test_section_units (test_section_id,ordinal,question_id)
		SELECT test_section_id,ordinal,question_id FROM app.test_section_questions WHERE test_section_id=$1 ORDER BY ordinal`, sectionID)
	return err
}
