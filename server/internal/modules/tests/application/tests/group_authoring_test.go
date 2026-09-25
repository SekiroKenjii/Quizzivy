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
	"quizzivy/internal/modules/tests/application/query"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/actor"
	"testing"
)

func TestGroupAuthoringCopiesFullContextAndKeepsBankLifecycleIndependent(t *testing.T) {
	ctx := context.Background()
	tx, author, repo := groupTransaction(t)
	tests := repositories.NewPostgres(db.NewContext(tx), adapters.GroupQuestions{}, mediarepo.NewPostgres(db.NewContext(tx))).WithGroupQuestions(adapters.GroupQuestions{})
	app := application.New(tests).WithGroups(repo, nil)
	who := actor.Actor{ID: author}
	bundle := storedGroupFixture(t, "")
	marker := groupIdentity(t)
	bundle.Group.Title = "Ngữ liệu 100% _ " + marker
	bundle.Questions[0].Input.Prompt = "Phát âm " + marker
	bundle.Questions[0].Input.Tags = []string{marker}
	bank, err := app.Commands.CreateGroup.Handle(ctx, command.CreateGroup{Bundle: bundle, Actor: who})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Commands.CreateGroup.Handle(ctx, command.CreateGroup{Bundle: bundle, Actor: who}); !errors.Is(err, domain.ErrGroupConflict) {
		t.Fatalf("duplicate graph identity: %v", err)
	}
	testID, section, updated := snapshotDraft(t, tx, author)
	copied, err := app.Commands.CopyGroup.Handle(ctx, command.CopyGroup{Mutation: model.GroupMutation{ID: bank.Bundle.Group.ID, ExpectedRevision: bank.Revision, ExpectedTestUpdatedAt: updated, Actor: who}, OwnerSectionID: &section})
	if err != nil {
		t.Fatal(err)
	}
	if copied.TestUpdatedAt == nil || copied.Bundle.Group.ID == bank.Bundle.Group.ID || len(copied.Bundle.Questions) != 2 {
		t.Fatalf("incomplete independent copy: %+v", copied)
	}
	draft, err := tests.Get(ctx, testID)
	if err != nil {
		t.Fatal(err)
	}
	if !draft.UpdatedAt.Equal(*copied.TestUpdatedAt) || len(draft.Sections[0].Units) != 2 || draft.Sections[0].Units[0].Kind != "question" || draft.Sections[0].Units[1].Kind != "group" || draft.Sections[0].Units[1].ID != copied.Bundle.Group.ID {
		t.Fatalf("missing coherent mixed outline: %+v", draft)
	}
	for _, term := range []string{"phat am " + marker, "100% _ " + marker} {
		found, err := app.Queries.Groups.Handle(ctx, query.Groups{Input: domain.GroupListInput{Query: term, Tag: marker}})
		if err != nil || found.Page.Total != 1 || len(found.Items) != 1 || found.Items[0].ID != bank.Bundle.Group.ID || found.Items[0].QuestionCount != 2 || found.Items[0].TotalPoints != "1.00" {
			t.Fatalf("bank filtering included section copy or lost accents: %+v %v", found, err)
		}
	}
	observed := model.GroupMutation{ID: bank.Bundle.Group.ID, ExpectedRevision: bank.Revision, Actor: who}
	changed := bank.Bundle
	changed.Questions[0].Input.Prompt = "Changed source"
	saved, err := app.Commands.UpdateGroup.Handle(ctx, command.UpdateGroup{Mutation: observed, Bundle: changed})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Commands.CopyGroup.Handle(ctx, command.CopyGroup{Mutation: observed}); !errors.Is(err, domain.ErrStaleWrite) {
		t.Fatalf("stale copy: %v", err)
	}
	observed.ExpectedRevision = saved.Revision
	if _, err := app.Commands.DeleteGroup.Handle(ctx, command.DeleteGroup{Mutation: observed}); !errors.Is(err, domain.ErrNotArchived) {
		t.Fatalf("unarchived deletion: %v", err)
	}
	archived, err := app.Commands.ArchiveGroup.Handle(ctx, command.ArchiveGroup{Mutation: observed, Archived: true})
	if err != nil {
		t.Fatal(err)
	}
	active, err := app.Queries.Groups.Handle(ctx, query.Groups{Input: domain.GroupListInput{Tag: marker}})
	if err != nil || active.Page.Total != 0 {
		t.Fatalf("archived group remained active: %+v %v", active, err)
	}
	archive, err := app.Queries.Groups.Handle(ctx, query.Groups{Input: domain.GroupListInput{Status: "archived", Tag: marker}})
	if err != nil || archive.Page.Total != 1 || len(archive.Items) != 1 || archive.Items[0].ID != bank.Bundle.Group.ID {
		t.Fatalf("archive filter: %+v %v", archive, err)
	}
	observed.ExpectedRevision = archived.Revision
	if _, err := app.Commands.DeleteGroup.Handle(ctx, command.DeleteGroup{Mutation: observed}); err != nil {
		t.Fatal(err)
	}
	untouched, err := app.Queries.Group.Handle(ctx, query.Group{ID: copied.Bundle.Group.ID})
	if err != nil {
		t.Fatal(err)
	}
	if untouched.Bundle.Questions[0].Input.Prompt == "Changed source" || untouched.TestUpdatedAt == nil {
		t.Fatal("source mutation/deletion changed copy")
	}
	if _, err := app.Commands.DeleteGroup.Handle(ctx, command.DeleteGroup{Mutation: model.GroupMutation{ID: untouched.Bundle.Group.ID, ExpectedRevision: untouched.Revision, ExpectedTestUpdatedAt: *untouched.TestUpdatedAt, Actor: who}}); err != nil {
		t.Fatal(err)
	}
	after, err := tests.Get(ctx, testID)
	if err != nil || after.QuestionCount != 1 || len(after.Sections[0].Units) != 1 || after.Sections[0].Units[0].Kind != "question" {
		t.Fatalf("section deletion left an orphan unit/member: %+v %v", after, err)
	}
}

func TestGroupAuthoringWithoutDependenciesFailsExplicitly(t *testing.T) {
	app := application.New(nil)
	_, err := app.Commands.CreateGroup.Handle(context.Background(), command.CreateGroup{})
	if !errors.Is(err, domain.ErrGroupUnavailable) {
		t.Fatalf("missing group writer: %v", err)
	}
	_, err = app.Queries.Group.Handle(context.Background(), query.Group{})
	if !errors.Is(err, domain.ErrGroupUnavailable) {
		t.Fatalf("missing group reader: %v", err)
	}
}
