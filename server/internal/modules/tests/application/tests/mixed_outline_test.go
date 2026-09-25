//go:build integration

package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/core/adapters"
	mediarepo "quizzivy/internal/modules/media/repositories"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"testing"
	"time"
)

func mixedSections(test domain.Test) []domain.SectionInput {
	out := make([]domain.SectionInput, len(test.Sections))
	for i, s := range test.Sections {
		out[i] = domain.SectionInput{ID: s.ID, Title: s.Title, Instructions: s.Instructions, QuestionIDs: append([]string{}, s.QuestionIDs...), SetUnits: true, Units: append([]domain.SectionUnit{}, s.Units...)}
	}
	return out
}

func TestMixedOutlineMovesFullGroupsIntoNewEmptySectionsAndPreservesRevision(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	author := makeAuthor(t, pool)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	testID, section, updated := snapshotDraft(t, tx, author)
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	media := mediarepo.NewPostgres(db.NewContext(pool))
	repo := repositories.NewPostgres(db.NewContext(pool), adapters.GroupQuestions{}, media).WithGroupQuestions(adapters.GroupQuestions{})
	groups := repositories.NewGroupsPostgres(db.NewContext(pool), adapters.GroupQuestions{}, media)
	t.Cleanup(func() {
		current, err := repo.Get(context.Background(), testID)
		if err != nil {
			t.Error(err)
			return
		}
		archived := domain.Archived
		if _, err := repo.Update(context.Background(), domain.UpdateRequest{ID: testID, ActorID: author, Now: time.Now(), Input: domain.UpdateInput{ExpectedUpdatedAt: current.UpdatedAt, Status: &archived}}); err != nil {
			t.Error(err)
			return
		}
		if err := repo.Delete(context.Background(), domain.Request{ID: testID, ActorID: author}, time.Now()); err != nil {
			t.Error(err)
		}
	})
	stored, err := groups.Create(ctx, domain.CreateGroupInput{Bundle: storedGroupFixture(t, ""), OwnerSectionID: &section, ExpectedTestUpdatedAt: updated, ActorID: author, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	emptyBundle := domain.GroupBundle{Group: domain.QuestionGroup{ID: groupIdentity(t), Title: "Empty"}}
	empty, err := groups.Create(ctx, domain.CreateGroupInput{Bundle: emptyBundle, OwnerSectionID: &section, ExpectedTestUpdatedAt: *stored.TestUpdatedAt, ActorID: author, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	before, err := repo.Get(ctx, testID)
	if err != nil {
		t.Fatal(err)
	}
	question := before.Sections[0].QuestionIDs[0]
	sections := []domain.SectionInput{
		{ID: section, Title: "Keep empty group", SetUnits: true, Units: []domain.SectionUnit{{Kind: "group", ID: empty.Bundle.Group.ID}}},
		{Title: "New destination", QuestionIDs: []string{question}, SetUnits: true, Units: []domain.SectionUnit{{Kind: "group", ID: stored.Bundle.Group.ID}, {Kind: "question", ID: question}}},
	}
	request := domain.UpdateRequest{ID: testID, ActorID: author, Now: time.Now(), Input: domain.UpdateInput{ExpectedUpdatedAt: before.UpdatedAt, GroupOutline: true, SetSections: true, Sections: sections}}
	after, err := repo.Update(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Sections) != 2 || after.QuestionCount != 3 || len(after.Sections[1].Units) != 2 || after.Sections[1].Units[0].ID != stored.Bundle.Group.ID {
		t.Fatalf("mixed order lost: %+v", after)
	}
	moved, err := groups.Get(ctx, stored.Bundle.Group.ID)
	if err != nil {
		t.Fatal(err)
	}
	if moved.OwnerSectionID == nil || *moved.OwnerSectionID != after.Sections[1].ID || moved.Revision != stored.Revision+1 || len(moved.Bundle.Questions) != 2 || !moved.TestUpdatedAt.Equal(after.UpdatedAt) {
		t.Fatalf("move tore aggregate: %+v", moved)
	}
	retained, err := groups.Get(ctx, empty.Bundle.Group.ID)
	if err != nil || retained.Revision != empty.Revision || *retained.OwnerSectionID != section {
		t.Fatalf("empty group lost or unrelated revision changed: %+v %v", retained, err)
	}
	if _, err := repo.Update(ctx, request); !errors.Is(err, domain.ErrStaleWrite) {
		t.Fatalf("stale outline: %v", err)
	}
	if err := groups.RemoveFromSection(ctx, domain.GroupMutation{ID: empty.Bundle.Group.ID, ExpectedRevision: empty.Revision, ExpectedTestUpdatedAt: after.UpdatedAt, ActorID: author, Now: time.Now()}); err != nil {
		t.Fatal(err)
	}
	current, err := repo.Get(ctx, testID)
	if err != nil {
		t.Fatal(err)
	}
	keep := mixedSections(current)[1:]
	saved, err := repo.Update(ctx, domain.UpdateRequest{ID: testID, ActorID: author, Now: time.Now(), Input: domain.UpdateInput{ExpectedUpdatedAt: current.UpdatedAt, GroupOutline: true, SetSections: true, Sections: keep}})
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Sections) != 1 || saved.Sections[0].Ordinal != 0 {
		t.Fatalf("old empty section not removed: %+v", saved.Sections)
	}
	version, err := repo.Publish(ctx, domain.PublishRequest{TestID: testID, ActorID: author}, time.Now(), domain.Publishing.Validate)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := repo.Preview(ctx, testID, version.Version)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Sections) != 1 || len(preview.Groups) != 1 || len(preview.Questions) != 3 || preview.Questions[2].Prompt != "Standalone" {
		t.Fatalf("publication order diverged: %+v", preview)
	}
}

func TestMixedOutlineRefusesMissingForeignAndDetachedMembersAtomically(t *testing.T) {
	ctx := context.Background()
	tx, author, groups := groupTransaction(t)
	testID, section, updated := snapshotDraft(t, tx, author)
	repo := repositories.NewPostgres(db.NewContext(tx), adapters.GroupQuestions{}, mediarepo.NewPostgres(db.NewContext(tx))).WithGroupQuestions(adapters.GroupQuestions{})
	stored, err := groups.Create(ctx, domain.CreateGroupInput{Bundle: storedGroupFixture(t, ""), OwnerSectionID: &section, ExpectedTestUpdatedAt: updated, ActorID: author, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := groups.Create(ctx, domain.CreateGroupInput{Bundle: storedGroupFixture(t, ""), ActorID: author, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	before, err := repo.Get(ctx, testID)
	if err != nil {
		t.Fatal(err)
	}
	changes := map[string]func([]domain.SectionInput){
		"omitted group":      func(s []domain.SectionInput) { s[0].Units = s[0].Units[:1] },
		"foreign bank group": func(s []domain.SectionInput) { s[0].Units[1].ID = foreign.Bundle.Group.ID },
		"detached child": func(s []domain.SectionInput) {
			id := stored.Bundle.Questions[0].ID
			s[0].QuestionIDs = append(s[0].QuestionIDs, id)
			s[0].Units = append(s[0].Units, domain.SectionUnit{Kind: "question", ID: id})
		},
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			sections := mixedSections(before)
			change(sections)
			title := "Must roll back"
			_, err := repo.Update(ctx, domain.UpdateRequest{ID: testID, ActorID: author, Now: time.Now(), Input: domain.UpdateInput{ExpectedUpdatedAt: before.UpdatedAt, Title: &title, GroupOutline: true, SetSections: true, Sections: sections}})
			if err == nil {
				t.Fatal("unsafe outline accepted")
			}
			after, err := repo.Get(ctx, testID)
			if err != nil || after.Title != before.Title || !after.UpdatedAt.Equal(before.UpdatedAt) || len(after.Sections[0].Units) != 2 {
				t.Fatalf("failed edit partially saved: %+v %v", after, err)
			}
		})
	}
	archived := domain.Archived
	saved, err := repo.Update(ctx, domain.UpdateRequest{ID: testID, ActorID: author, Now: time.Now(), Input: domain.UpdateInput{ExpectedUpdatedAt: before.UpdatedAt, Status: &archived}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Update(ctx, domain.UpdateRequest{ID: testID, ActorID: author, Now: time.Now(), Input: domain.UpdateInput{ExpectedUpdatedAt: saved.UpdatedAt, GroupOutline: true, SetSections: true, Sections: mixedSections(saved)}}); !errors.Is(err, domain.ErrArchived) {
		t.Fatalf("archived mixed write: %v", err)
	}
}

func TestLegacyEditAfterLastGroupRemovalClearsObsoleteUnits(t *testing.T) {
	ctx := context.Background()
	tx, author, groups := groupTransaction(t)
	testID, section, updated := snapshotDraft(t, tx, author)
	repo := repositories.NewPostgres(db.NewContext(tx), adapters.GroupQuestions{}, mediarepo.NewPostgres(db.NewContext(tx)))
	group, err := groups.Create(ctx, domain.CreateGroupInput{Bundle: storedGroupFixture(t, ""), OwnerSectionID: &section, ExpectedTestUpdatedAt: updated, ActorID: author, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if err := groups.RemoveFromSection(ctx, domain.GroupMutation{ID: group.Bundle.Group.ID, ExpectedRevision: group.Revision, ExpectedTestUpdatedAt: *group.TestUpdatedAt, ActorID: author, Now: time.Now()}); err != nil {
		t.Fatal(err)
	}
	before, err := repo.Get(ctx, testID)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := repo.Update(ctx, domain.UpdateRequest{ID: testID, ActorID: author, Now: time.Now(), Input: domain.UpdateInput{ExpectedUpdatedAt: before.UpdatedAt, SetSections: true, Sections: []domain.SectionInput{{ID: section, Title: "Empty"}}}})
	if err != nil || saved.QuestionCount != 0 || len(saved.Sections[0].Units) != 0 {
		t.Fatalf("obsolete units survived legacy edit: %+v %v", saved, err)
	}
}
