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
	"quizzivy/internal/shared/access"
	"testing"
	"time"
)

func TestPublishingADraftWhoseQuestionNamesADeletedAssetIsRefusedNamingTheQuestion(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	author := pubMakeAuthor(t, pool)
	b := newBuilder(t, pool, author)

	gone := audioAsset(t, pool, author, "missing-media-gone")
	kept := audioAsset(t, pool, author, "missing-media-kept")
	first := b.listeningQuestion("Nghe tệp đã xoá", gone)
	second := b.listeningQuestion("Nghe tệp còn lại", kept)
	draft := b.draft("Đề có một tệp đã xoá", first, second)

	deleted, err := pool.Exec(ctx, `UPDATE app.media_assets SET deleted_at = now() WHERE id = $1`, gone)
	if err != nil {
		t.Fatal(err)
	}
	if deleted.RowsAffected() != 1 {
		t.Fatalf("soft-deleting the asset touched %d rows, want 1", deleted.RowsAffected())
	}

	_, err = b.publish(draft.ID)
	var invalid *domain.PublishValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("publish returned %v, want a PublishValidationError", err)
	}
	if len(invalid.Violations) != 1 {
		t.Fatalf("got %d violations, want 1: %+v", len(invalid.Violations), invalid.Violations)
	}
	violation := invalid.Violations[0]
	if violation.Rule != domain.AudioQuestionHasAsset || violation.QuestionID != first || violation.SectionID == "" {
		t.Errorf("violation is %+v, want %s on question %s in its section", violation, domain.AudioQuestionHasAsset, first)
	}

	var versions int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM app.test_versions WHERE test_id = $1`, draft.ID).Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if versions != 0 {
		t.Errorf("the refused publish left %d version(s)", versions)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status::text FROM app.tests WHERE id = $1`, draft.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "draft" {
		t.Errorf("the refused publish left the test %s, want draft", status)
	}

	restored, err := pool.Exec(ctx, `UPDATE app.media_assets SET deleted_at = NULL WHERE id = $1`, gone)
	if err != nil {
		t.Fatal(err)
	}
	if restored.RowsAffected() != 1 {
		t.Fatalf("restoring the asset touched %d rows, want 1", restored.RowsAffected())
	}
	if _, err := b.publish(draft.ID); err != nil {
		t.Fatalf("publish with the asset restored: %v", err)
	}
}

func TestDuplicatingADraftWhoseGroupNamesADeletedAssetIsNotAPublishRefusal(t *testing.T) {
	ctx := context.Background()
	tx, author, groups := groupTransaction(t)
	testID, section, updated := snapshotDraft(t, tx, author)
	asset := storedGroupAsset(t, tx, author, "audio")
	if _, err := groups.Create(ctx, domain.CreateGroupInput{Bundle: storedGroupFixture(t, asset), OwnerSectionID: &section, ExpectedTestUpdatedAt: updated, ActorID: author, Now: time.Now(), Scope: access.Scope{UserID: author}, Grants: bothKeys}); err != nil {
		t.Fatal(err)
	}

	deleted, err := tx.Exec(ctx, `UPDATE app.media_assets SET deleted_at = now() WHERE id = $1`, asset)
	if err != nil {
		t.Fatal(err)
	}
	if deleted.RowsAffected() != 1 {
		t.Fatalf("soft-deleting the asset touched %d rows, want 1", deleted.RowsAffected())
	}

	media := mediarepo.NewPostgres(db.NewContext(tx))
	repo := repositories.NewPostgres(db.NewContext(tx), adapters.GroupQuestions{}, media).WithGroupQuestions(adapters.GroupQuestions{})
	_, err = repo.Duplicate(ctx, domain.DuplicateInput{ID: testID, ActorID: author, Now: time.Now(), Scope: access.Scope{UserID: author}})
	if err == nil {
		t.Fatal("duplicating copied a group whose material names a deleted asset")
	}
	var invalid *domain.PublishValidationError
	if errors.As(err, &invalid) {
		t.Fatalf("duplicate answered a publish refusal: %v", err)
	}
}
