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

func TestRestoringADraftOverAGroupMemberAnotherDraftHoldsIsRefusedAsReferenced(t *testing.T) {
	ctx := context.Background()
	tx, author, groups := groupTransaction(t)
	testID, section, updated := snapshotDraft(t, tx, author)
	asset := storedGroupAsset(t, tx, author, "audio")
	owned, err := groups.Create(ctx, domain.CreateGroupInput{Bundle: storedGroupFixture(t, asset), OwnerSectionID: &section, ExpectedTestUpdatedAt: updated, ActorID: author, Now: time.Now(), Scope: access.Scope{UserID: author}, Grants: bothKeys})
	if err != nil {
		t.Fatal(err)
	}
	media := mediarepo.NewPostgres(db.NewContext(tx))
	repo := repositories.NewPostgres(db.NewContext(tx), adapters.GroupQuestions{}, media).WithGroupQuestions(adapters.GroupQuestions{})
	if _, err := repo.Publish(ctx, domain.PublishRequest{TestID: testID, ActorID: author, Scope: access.Scope{UserID: author}}, time.Now(), domain.Publishing.Validate); err != nil {
		t.Fatal(err)
	}
	_, holder, _ := snapshotDraft(t, tx, author)
	held, err := tx.Exec(ctx, `INSERT INTO app.test_section_questions (test_section_id, ordinal, question_id)
		SELECT $1::uuid, coalesce((SELECT max(ordinal) + 1 FROM app.test_section_questions WHERE test_section_id = $1::uuid), 0), q.id
		FROM app.questions q WHERE q.context_group_id = $2::uuid LIMIT 1`, holder, owned.Bundle.Group.ID)
	if err != nil {
		t.Fatal(err)
	}
	if held.RowsAffected() != 1 {
		t.Fatalf("the other draft holds %d members, want 1", held.RowsAffected())
	}
	req := domain.VersionRequest{Request: domain.Request{ID: testID, ActorID: author, Scope: access.Scope{UserID: author}}, Version: 1}
	if err := tx.QueryRow(ctx, `SELECT updated_at FROM app.tests WHERE id=$1`, testID).Scan(&req.ExpectedUpdatedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateDraftFromVersion(ctx, req, time.Now()); !errors.Is(err, domain.ErrDraftReferenced) {
		t.Fatalf("restoring over a group whose member another draft references answered %v, want domain.ErrDraftReferenced", err)
	}
	kept, err := groups.Get(ctx, everyone, owned.Bundle.Group.ID)
	if err != nil || kept.Revision != owned.Revision {
		t.Fatalf("the refused restore left the draft's group at revision %d, want %d: %v", kept.Revision, owned.Revision, err)
	}
	var members int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.questions WHERE context_group_id = $1`, owned.Bundle.Group.ID).Scan(&members); err != nil || members != len(owned.Bundle.Questions) {
		t.Fatalf("the refused restore left %d members, want %d: %v", members, len(owned.Bundle.Questions), err)
	}
	var placed int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.test_section_units WHERE group_id = $1`, owned.Bundle.Group.ID).Scan(&placed); err != nil || placed != 1 {
		t.Fatalf("the refused restore left the group in %d section units, want 1: %v", placed, err)
	}
	var audited int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE action = 'test.draft_restored' AND entity_id = $1`, testID).Scan(&audited); err != nil || audited != 0 {
		t.Fatalf("the refused restore wrote %d audit rows, want 0: %v", audited, err)
	}
}
