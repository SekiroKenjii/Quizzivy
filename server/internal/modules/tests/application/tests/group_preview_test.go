//go:build integration

package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"quizzivy/internal/core/adapters"
	mediarepo "quizzivy/internal/modules/media/repositories"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"strings"
	"testing"
	"time"
)

func TestGroupPreviewKeepsFrozenContextAndDefaultVersion(t *testing.T) {
	ctx := context.Background()
	tx, author, groups := groupTransaction(t)
	testID, section, updated := snapshotDraft(t, tx, author)
	asset := storedGroupAsset(t, tx, author, "audio")
	source := storedGroupFixture(t, asset)
	stored, err := groups.Create(ctx, domain.CreateGroupInput{Bundle: source, OwnerSectionID: &section, ExpectedTestUpdatedAt: updated, ActorID: author, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	media := mediarepo.NewPostgres(db.NewContext(tx))
	repo := repositories.NewPostgres(db.NewContext(tx), adapters.GroupQuestions{}, media).WithGroupQuestions(adapters.GroupQuestions{})
	if _, err := repo.Publish(ctx, domain.PublishRequest{TestID: testID, ActorID: author}, time.Now(), domain.Publishing.Validate); err != nil {
		t.Fatal(err)
	}
	original, err := repo.Preview(ctx, testID, 0)
	if err != nil || len(original.Groups) != 1 || len(original.Sections) != 1 || len(original.Questions) != 3 {
		t.Fatalf("incomplete preview: %+v, %v", original, err)
	}
	group := original.Groups[0]
	if group.ID == source.Group.ID || group.QuestionIDs[0] != original.Questions[1].ID || group.QuestionIDs[1] != original.Questions[2].ID || group.SectionID != original.Sections[0].ID {
		t.Fatal("preview broke frozen identity/order")
	}
	if len(group.Stimuli) != 2 || len(group.AssetIDs) != 1 || group.AssetIDs[0] != asset || len(group.Recordings) != 1 || *group.Recordings[0].Policy.MaxPlays != 2 {
		t.Fatal("preview lost shared material or recording policy")
	}
	var bound *domain.GroupGapBinding
	for _, gap := range group.Stimuli[0].Gaps {
		if gap.Kind == "blank" {
			bound = &gap
		}
	}
	if bound == nil || bound.QuestionID != group.QuestionIDs[1] || *bound.BlankGapID != *original.Questions[2].Blanks[0].GapID {
		t.Fatal("cloze target did not resolve to frozen blank")
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"Lời thoại riêng", "chính xác", "IsCorrect", "AcceptedAnswers", "SampleAnswer", "\"Transcript\""} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("learner projection exposes %s", secret)
		}
	}
	stored.Bundle.Group.Title = "Changed context"
	mutation := groupMutation(stored, author)
	if err := tx.QueryRow(ctx, `SELECT updated_at FROM app.tests WHERE id=$1`, testID).Scan(&mutation.ExpectedTestUpdatedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := groups.Update(ctx, domain.UpdateGroupInput{GroupMutation: mutation, Bundle: stored.Bundle}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Publish(ctx, domain.PublishRequest{TestID: testID, ActorID: author}, time.Now(), domain.Publishing.Validate); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE app.tests SET current_version=1 WHERE id=$1`, testID); err != nil {
		t.Fatal(err)
	}
	current, err := repo.Preview(ctx, testID, 0)
	if err != nil || current.Version != 1 || current.Groups[0].Title != source.Group.Title {
		t.Fatalf("default preview uses latest instead: %+v, %v", current, err)
	}
	newer, err := repo.Preview(ctx, testID, 2)
	if err != nil || newer.Version != 2 || newer.Groups[0].Title != "Changed context" {
		t.Fatalf("explicit version unavailable: %+v, %v", newer, err)
	}
	if _, err := repo.Preview(ctx, testID, 99); !errors.Is(err, domain.ErrNotPublished) {
		t.Fatalf("missing version: %v", err)
	}
}
