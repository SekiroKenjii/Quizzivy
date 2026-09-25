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
	"slices"
	"testing"
	"time"
)

func TestDraftSummariesIncludeOwnedMembersAndProtectLegacyOutlineWrites(t *testing.T) {
	ctx := context.Background()
	tx, author, groups := groupTransaction(t)
	testID, section, updated := snapshotDraft(t, tx, author)
	asset := storedGroupAsset(t, tx, author, "audio")
	bundle := storedGroupFixture(t, asset)
	tag := "shared-" + testID
	bundle.Questions[0].Input.Tags = []string{tag}
	stored, err := groups.Create(ctx, domain.CreateGroupInput{Bundle: bundle, OwnerSectionID: &section, ExpectedTestUpdatedAt: updated, ActorID: author, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := groups.Create(ctx, domain.CreateGroupInput{Bundle: storedGroupFixture(t, asset), ActorID: author, Now: time.Now()}); err != nil {
		t.Fatal(err)
	}
	repo := repositories.NewPostgres(db.NewContext(tx), adapters.GroupQuestions{}, mediarepo.NewPostgres(db.NewContext(tx))).WithGroupQuestions(adapters.GroupQuestions{})
	draft, err := repo.Get(ctx, testID)
	if err != nil {
		t.Fatal(err)
	}
	if draft.QuestionCount != 3 || draft.AudioCount != 2 || draft.TotalPoints != "2.00" {
		t.Fatalf("incomplete or duplicated summary: %+v", draft)
	}
	found, page, err := repo.List(ctx, domain.ListInput{Tags: []string{tag}})
	if err != nil || page.Total != 1 || len(found) != 1 || found[0].ID != testID || found[0].QuestionCount != 3 || found[0].AudioCount != 2 {
		t.Fatalf("group-tag list lost summary: %+v %+v %v", found, page, err)
	}
	tags, err := repo.Tags(ctx, domain.ListInput{})
	if err != nil || !slices.Contains(tags, tag) {
		t.Fatalf("group tag absent from filter: %v %v", tags, err)
	}
	changedTitle := "must not save"
	_, err = repo.Update(ctx, domain.UpdateRequest{ID: testID, ActorID: author, Now: time.Now(), Input: domain.UpdateInput{ExpectedUpdatedAt: draft.UpdatedAt, Title: &changedTitle, SetSections: true, Sections: []domain.SectionInput{}}})
	if !errors.Is(err, domain.ErrGroupOutlineRequired) {
		t.Fatalf("legacy writer accepted grouped outline: %v", err)
	}
	unchanged, err := repo.Get(ctx, testID)
	if err != nil || unchanged.Title != draft.Title || !unchanged.UpdatedAt.Equal(draft.UpdatedAt) {
		t.Fatalf("failed outline write partially saved: %+v %v", unchanged, err)
	}
	if remaining, err := groups.Get(ctx, stored.Bundle.Group.ID); err != nil || remaining.Revision != stored.Revision {
		t.Fatalf("failed write changed graph: %+v %v", remaining, err)
	}
	changedTitle = "metadata still editable"
	if _, err = repo.Update(ctx, domain.UpdateRequest{ID: testID, ActorID: author, Now: time.Now(), Input: domain.UpdateInput{ExpectedUpdatedAt: draft.UpdatedAt, Title: &changedTitle}}); err != nil {
		t.Fatal(err)
	}
	published, err := repo.Publish(ctx, domain.PublishRequest{TestID: testID, ActorID: author}, time.Now(), domain.Publishing.Validate)
	if err != nil || published.QuestionCount != draft.QuestionCount || published.TotalPoints != draft.TotalPoints {
		t.Fatalf("draft/published summary diverged: %+v %v", published, err)
	}
	versions, err := repo.ListVersions(ctx, testID)
	if err != nil || len(versions) != 1 || versions[0].AudioCount != draft.AudioCount {
		t.Fatalf("draft/published listening count diverged: %+v %v", versions, err)
	}
}

func TestArchivedVersionChangesFailBeforeRewritingFlatOrGroupedDrafts(t *testing.T) {
	for _, grouped := range []bool{false, true} {
		t.Run(map[bool]string{false: "flat", true: "grouped"}[grouped], func(t *testing.T) {
			ctx := context.Background()
			tx, author, groups := groupTransaction(t)
			testID, section, updated := snapshotDraft(t, tx, author)
			if grouped {
				if _, err := groups.Create(ctx, domain.CreateGroupInput{Bundle: storedGroupFixture(t, ""), OwnerSectionID: &section, ExpectedTestUpdatedAt: updated, ActorID: author, Now: time.Now()}); err != nil {
					t.Fatal(err)
				}
			}
			repo := repositories.NewPostgres(db.NewContext(tx), adapters.GroupQuestions{}, mediarepo.NewPostgres(db.NewContext(tx))).WithGroupQuestions(adapters.GroupQuestions{})
			for range 2 {
				if _, err := repo.Publish(ctx, domain.PublishRequest{TestID: testID, ActorID: author}, time.Now(), domain.Publishing.Validate); err != nil {
					t.Fatal(err)
				}
			}
			draft, err := repo.Get(ctx, testID)
			if err != nil {
				t.Fatal(err)
			}
			archived, err := repo.Update(ctx, domain.UpdateRequest{ID: testID, ActorID: author, Now: time.Now(), Input: domain.UpdateInput{ExpectedUpdatedAt: draft.UpdatedAt, Status: groupValue(domain.Archived)}})
			if err != nil {
				t.Fatal(err)
			}
			req := domain.VersionRequest{Request: domain.Request{ID: testID, ActorID: author}, Version: 1, ExpectedUpdatedAt: archived.UpdatedAt}
			if _, err := repo.CreateDraftFromVersion(ctx, req, time.Now()); !errors.Is(err, domain.ErrArchived) {
				t.Fatalf("archived restore: %v", err)
			}
			if _, err := repo.SetCurrentVersion(ctx, req, time.Now()); !errors.Is(err, domain.ErrArchived) {
				t.Fatalf("archived default change: %v", err)
			}
			after, err := repo.Get(ctx, testID)
			if err != nil || after.CurrentVersion != 2 || after.QuestionCount != draft.QuestionCount || !after.UpdatedAt.Equal(archived.UpdatedAt) {
				t.Fatalf("archived rejection changed draft: %+v %v", after, err)
			}
			if err := repo.DeleteVersion(ctx, req, time.Now()); err != nil {
				t.Fatalf("archived unused version deletion: %v", err)
			}
			after, err = repo.Get(ctx, testID)
			if err != nil {
				t.Fatal(err)
			}
			active, err := repo.Update(ctx, domain.UpdateRequest{ID: testID, ActorID: author, Now: time.Now(), Input: domain.UpdateInput{ExpectedUpdatedAt: after.UpdatedAt, Status: groupValue(domain.Published)}})
			if err != nil {
				t.Fatal(err)
			}
			req.ExpectedUpdatedAt = active.UpdatedAt
			req.Version = 2
			restored, err := repo.CreateDraftFromVersion(ctx, req, time.Now())
			if err != nil || restored.QuestionCount != draft.QuestionCount {
				t.Fatalf("unarchived restore: %+v %v", restored, err)
			}
		})
	}
}
