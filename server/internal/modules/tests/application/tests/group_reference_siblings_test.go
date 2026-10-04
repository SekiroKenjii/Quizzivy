//go:build integration

package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/core/adapters"
	mediarepo "quizzivy/internal/modules/media/repositories"
	questions "quizzivy/internal/modules/questions/domain"
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

func TestDeletingATestWhoseGroupMemberAnotherDraftHoldsIsRefusedAsReferenced(t *testing.T) {
	ctx := context.Background()
	tx, author, repo := groupTransaction(t)
	tests := repositories.NewPostgres(db.NewContext(tx), adapters.GroupQuestions{}, mediarepo.NewPostgres(db.NewContext(tx))).WithGroupQuestions(adapters.GroupQuestions{})
	app := application.New(tests).WithGroups(repo, nil)
	who := actor.Actor{ID: author, Scope: access.Scope{UserID: author}}
	bank, err := app.Commands.CreateGroup.Handle(ctx, command.CreateGroup{Bundle: storedGroupFixture(t, ""), Actor: who, Grants: bothKeys})
	if err != nil {
		t.Fatal(err)
	}
	doomed, section, updated := snapshotDraft(t, tx, author)
	copied, err := app.Commands.CopyGroup.Handle(ctx, command.CopyGroup{Mutation: model.GroupMutation{ID: bank.Bundle.Group.ID, ExpectedRevision: bank.Revision, ExpectedTestUpdatedAt: updated, Actor: who, Grants: bothKeys}, OwnerSectionID: &section})
	if err != nil {
		t.Fatal(err)
	}
	if copied.TestUpdatedAt == nil {
		t.Fatalf("the section copy carries no draft revision: %+v", copied)
	}
	_, holder, _ := snapshotDraft(t, tx, author)
	held, err := tx.Exec(ctx, `INSERT INTO app.test_section_questions (test_section_id, ordinal, question_id)
		SELECT $1::uuid, coalesce((SELECT max(ordinal) + 1 FROM app.test_section_questions WHERE test_section_id = $1::uuid), 0), q.id
		FROM app.questions q WHERE q.context_group_id = $2::uuid LIMIT 1`, holder, copied.Bundle.Group.ID)
	if err != nil {
		t.Fatal(err)
	}
	if held.RowsAffected() != 1 {
		t.Fatalf("the other draft holds %d members, want 1", held.RowsAffected())
	}
	archived := domain.Archived
	if _, err := app.Commands.Update.Handle(ctx, command.Update{Request: reqFor(doomed, author), Input: domain.UpdateInput{ExpectedUpdatedAt: *copied.TestUpdatedAt, Status: &archived}}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Commands.Delete.Handle(ctx, command.Delete{Request: reqFor(doomed, author)}); !errors.Is(err, domain.ErrReferenced) {
		t.Fatalf("deleting a test whose group member another draft references answered %v, want domain.ErrReferenced", err)
	}
	var kept int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.tests WHERE id = $1`, doomed).Scan(&kept); err != nil || kept != 1 {
		t.Fatalf("the refused delete left %d tests, want 1: %v", kept, err)
	}
	var groups int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.question_groups WHERE id = $1`, copied.Bundle.Group.ID).Scan(&groups); err != nil || groups != 1 {
		t.Fatalf("the refused delete left %d groups, want 1: %v", groups, err)
	}
}

func TestUpdatingAGroupToDropAMemberADraftStillHoldsIsRefusedAsAGroupReference(t *testing.T) {
	ctx := context.Background()
	tx, author, repo := groupTransaction(t)
	before := createStoredFixture(t, tx, author, repo)
	kept, dropped := before.Bundle.Questions[0].ID, before.Bundle.Questions[1].ID
	_, section, _ := snapshotDraft(t, tx, author)
	held, err := tx.Exec(ctx, `INSERT INTO app.test_section_questions (test_section_id, ordinal, question_id)
		SELECT $1::uuid, coalesce((SELECT max(ordinal) + 1 FROM app.test_section_questions WHERE test_section_id = $1::uuid), 0), q.id
		FROM app.questions q WHERE q.context_group_id = $2::uuid AND q.id = $3::uuid`, section, before.Bundle.Group.ID, dropped)
	if err != nil {
		t.Fatal(err)
	}
	if held.RowsAffected() != 1 {
		t.Fatalf("the draft holds %d members, want 1", held.RowsAffected())
	}
	added := groupIdentity(t)
	bundle := domain.GroupBundle{
		Group: domain.QuestionGroup{ID: before.Bundle.Group.ID, Title: "Replaced", Members: []domain.GroupMember{{QuestionID: kept, OptionOrder: "shuffle"}, {QuestionID: added, OptionOrder: "fixed"}}},
		Questions: []domain.GroupQuestion{
			{ID: kept, Input: questions.Input{Type: questions.ShortAnswer, Prompt: "New response", Points: "2"}},
			{ID: added, Input: questions.Input{Type: questions.SingleChoice, Prompt: "New choice", Points: "1", Options: []questions.OptionInput{{Text: "A", IsCorrect: true}, {Text: "B"}}}},
		},
	}
	_, err = repo.Update(ctx, domain.UpdateGroupInput{GroupMutation: groupMutation(before, author), Bundle: bundle})
	var refused *domain.GroupError
	if !errors.As(err, &refused) || refused.Rule != "group_reference" {
		t.Fatalf("dropping a member a draft still references answered %v, want a group_reference refusal", err)
	}
	var members int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.questions WHERE id = $1`, dropped).Scan(&members); err != nil || members != 1 {
		t.Fatalf("the refused update left %d of the dropped member, want 1: %v", members, err)
	}
	var revision int64
	if err := tx.QueryRow(ctx, `SELECT revision FROM app.question_groups WHERE id = $1`, before.Bundle.Group.ID).Scan(&revision); err != nil || revision != before.Revision {
		t.Fatalf("the refused update left revision %d, want %d: %v", revision, before.Revision, err)
	}
}
