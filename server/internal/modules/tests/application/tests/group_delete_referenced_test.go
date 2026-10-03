//go:build integration

package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/core/adapters"
	mediarepo "quizzivy/internal/modules/media/repositories"
	"quizzivy/internal/modules/tests/application"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/application/model"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
	"testing"
)

func TestDeletingAGroupWhoseMemberADraftStillHoldsIsRefusedAsReferenced(t *testing.T) {
	ctx := context.Background()
	tx, author, repo := groupTransaction(t)
	tests := repositories.NewPostgres(db.NewContext(tx), adapters.GroupQuestions{}, mediarepo.NewPostgres(db.NewContext(tx))).WithGroupQuestions(adapters.GroupQuestions{})
	app := application.New(tests).WithGroups(repo, nil)
	who := actor.Actor{ID: author, Scope: access.Scope{UserID: author}}
	bank, err := app.Commands.CreateGroup.Handle(ctx, command.CreateGroup{Bundle: storedGroupFixture(t, ""), Actor: who, Grants: bothKeys})
	if err != nil {
		t.Fatal(err)
	}
	observed := model.GroupMutation{ID: bank.Bundle.Group.ID, ExpectedRevision: bank.Revision, Actor: who, Grants: bothKeys}
	archived, err := app.Commands.ArchiveGroup.Handle(ctx, command.ArchiveGroup{Mutation: observed, Archived: true})
	if err != nil {
		t.Fatal(err)
	}
	_, section, _ := snapshotDraft(t, tx, author)
	held, err := tx.Exec(ctx, `INSERT INTO app.test_section_questions (test_section_id, ordinal, question_id)
		SELECT $1::uuid, coalesce((SELECT max(ordinal) + 1 FROM app.test_section_questions WHERE test_section_id = $1::uuid), 0), q.id
		FROM app.questions q WHERE q.context_group_id = $2::uuid LIMIT 1`, section, bank.Bundle.Group.ID)
	if err != nil {
		t.Fatal(err)
	}
	if held.RowsAffected() != 1 {
		t.Fatalf("the draft holds %d members, want 1", held.RowsAffected())
	}
	observed.ExpectedRevision = archived.Revision
	if _, err := app.Commands.DeleteGroup.Handle(ctx, command.DeleteGroup{Mutation: observed}); !errors.Is(err, domain.ErrReferenced) {
		t.Fatalf("deleting a group a draft still references answered %v, want domain.ErrReferenced", err)
	}
	var groups int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.question_groups WHERE id = $1`, bank.Bundle.Group.ID).Scan(&groups); err != nil || groups != 1 {
		t.Fatalf("the refused delete left %d groups, want 1: %v", groups, err)
	}
}

func TestRemovingASectionGroupWhoseMemberADraftStillHoldsIsRefusedAsReferenced(t *testing.T) {
	ctx := context.Background()
	tx, author, repo := groupTransaction(t)
	tests := repositories.NewPostgres(db.NewContext(tx), adapters.GroupQuestions{}, mediarepo.NewPostgres(db.NewContext(tx))).WithGroupQuestions(adapters.GroupQuestions{})
	app := application.New(tests).WithGroups(repo, nil)
	who := actor.Actor{ID: author, Scope: access.Scope{UserID: author}}
	bank, err := app.Commands.CreateGroup.Handle(ctx, command.CreateGroup{Bundle: storedGroupFixture(t, ""), Actor: who, Grants: bothKeys})
	if err != nil {
		t.Fatal(err)
	}
	_, section, updated := snapshotDraft(t, tx, author)
	copied, err := app.Commands.CopyGroup.Handle(ctx, command.CopyGroup{Mutation: model.GroupMutation{ID: bank.Bundle.Group.ID, ExpectedRevision: bank.Revision, ExpectedTestUpdatedAt: updated, Actor: who, Grants: bothKeys}, OwnerSectionID: &section})
	if err != nil {
		t.Fatal(err)
	}
	if copied.TestUpdatedAt == nil {
		t.Fatalf("the section copy carries no draft revision: %+v", copied)
	}
	held, err := tx.Exec(ctx, `INSERT INTO app.test_section_questions (test_section_id, ordinal, question_id)
		SELECT $1::uuid, coalesce((SELECT max(ordinal) + 1 FROM app.test_section_questions WHERE test_section_id = $1::uuid), 0), q.id
		FROM app.questions q WHERE q.context_group_id = $2::uuid LIMIT 1`, section, copied.Bundle.Group.ID)
	if err != nil {
		t.Fatal(err)
	}
	if held.RowsAffected() != 1 {
		t.Fatalf("the draft holds %d members, want 1", held.RowsAffected())
	}
	observed := model.GroupMutation{ID: copied.Bundle.Group.ID, ExpectedRevision: copied.Revision, ExpectedTestUpdatedAt: *copied.TestUpdatedAt, Actor: who, Grants: bothKeys}
	if _, err := app.Commands.DeleteGroup.Handle(ctx, command.DeleteGroup{Mutation: observed}); !errors.Is(err, domain.ErrReferenced) {
		t.Fatalf("removing a section group a draft still references answered %v, want domain.ErrReferenced", err)
	}
	var units int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.test_section_units WHERE group_id = $1`, copied.Bundle.Group.ID).Scan(&units); err != nil || units != 1 {
		t.Fatalf("the refused removal left %d units for the group, want 1: %v", units, err)
	}
}
