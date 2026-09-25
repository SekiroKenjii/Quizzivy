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

func TestRestoreVersionReplacesWholeGroupAndRemapsBothGapEnds(t *testing.T) {
	ctx := context.Background()
	tx, author, groups := groupTransaction(t)
	testID, section, updated := snapshotDraft(t, tx, author)
	asset := storedGroupAsset(t, tx, author, "audio")
	original, err := groups.Create(ctx, domain.CreateGroupInput{Bundle: storedGroupFixture(t, asset), OwnerSectionID: &section, ExpectedTestUpdatedAt: updated, ActorID: author, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	media := mediarepo.NewPostgres(db.NewContext(tx))
	repo := repositories.NewPostgres(db.NewContext(tx), adapters.GroupQuestions{}, media).WithGroupQuestions(adapters.GroupQuestions{})
	published, err := repo.Publish(ctx, domain.PublishRequest{TestID: testID, ActorID: author}, time.Now(), domain.Publishing.Validate)
	if err != nil {
		t.Fatal(err)
	}
	before := frozenGroupDigest(t, tx, published.ID)
	original.Bundle.Group.Recordings[0].Policy.MaxPlays = groupValue(9)
	original.Bundle.Group.Stimuli[0].Title = "Changed draft"
	mutation := groupMutation(original, author)
	if err := tx.QueryRow(ctx, `SELECT updated_at FROM app.tests WHERE id=$1`, testID).Scan(&mutation.ExpectedTestUpdatedAt); err != nil {
		t.Fatal(err)
	}
	changed, err := groups.Update(ctx, domain.UpdateGroupInput{GroupMutation: mutation, Bundle: original.Bundle})
	if err != nil {
		t.Fatal(err)
	}
	req := domain.VersionRequest{Request: domain.Request{ID: testID, ActorID: author}, Version: 1}
	if err := tx.QueryRow(ctx, `SELECT updated_at FROM app.tests WHERE id=$1`, testID).Scan(&req.ExpectedUpdatedAt); err != nil {
		t.Fatal(err)
	}
	missingPort := repositories.NewPostgres(db.NewContext(tx), adapters.GroupQuestions{}, media)
	if _, err := missingPort.CreateDraftFromVersion(ctx, req, time.Now()); err == nil {
		t.Fatal("restore without group writer silently detached members")
	}
	preserved, err := groups.Get(ctx, changed.Bundle.Group.ID)
	if err != nil || preserved.Revision != changed.Revision {
		t.Fatalf("failed restore deleted current draft: %+v, %v", preserved, err)
	}
	previous := changed.Bundle.Group.ID
	for range 2 {
		restored, err := repo.CreateDraftFromVersion(ctx, req, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		req.ExpectedUpdatedAt = restored.UpdatedAt
		var id string
		if err := tx.QueryRow(ctx, `SELECT g.id::text FROM app.question_groups g JOIN app.test_sections s ON s.id=g.owner_section_id WHERE s.test_id=$1`, testID).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if id == previous {
			t.Fatal("restore reused the editable source group")
		}
		if _, err := groups.Get(ctx, previous); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("restore stranded old owned graph: %v", err)
		}
		copy, err := groups.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if len(copy.Bundle.Questions) != 2 || *copy.Bundle.Group.Recordings[0].Policy.MaxPlays != 2 || copy.Bundle.Group.Stimuli[0].Title == "Changed draft" {
			t.Fatalf("restore used mutable draft content: %+v", copy)
		}
		if *copy.Bundle.Questions[1].Input.Blanks[0].GapID == *original.Bundle.Questions[1].Input.Blanks[0].GapID {
			t.Fatal("restore retained source gap identity")
		}
		if err := copy.Bundle.ValidateForPublish(false); err != nil {
			t.Fatal(err)
		}
		previous = id
	}
	if after := frozenGroupDigest(t, tx, published.ID); after != before {
		t.Fatal("restoring changed published context")
	}
	republished, err := repo.Publish(ctx, domain.PublishRequest{TestID: testID, ActorID: author}, time.Now(), domain.Publishing.Validate)
	if err != nil || republished.QuestionCount != published.QuestionCount || republished.TotalPoints != published.TotalPoints {
		t.Fatalf("restored graph did not republish coherently: %+v, %v", republished, err)
	}
	if _, err := tx.Exec(ctx, `SET CONSTRAINTS ALL IMMEDIATE`); err != nil {
		t.Fatal(err)
	}
}
