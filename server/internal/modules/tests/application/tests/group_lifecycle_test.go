//go:build integration

package application_test

import (
	"context"
	"encoding/json"
	"errors"
	mediadomain "quizzivy/internal/modules/media/domain"
	mediarepo "quizzivy/internal/modules/media/repositories"
	questions "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func groupMutation(group domain.StoredGroup, author string) domain.GroupMutation {
	return domain.GroupMutation{ID: group.Bundle.Group.ID, ExpectedRevision: group.Revision, ActorID: author, Now: time.Now()}
}

func createStoredFixture(t *testing.T, tx pgx.Tx, author string, repo *repositories.GroupsPostgres) domain.StoredGroup {
	t.Helper()
	asset := storedGroupAsset(t, tx, author, "audio")
	stored, err := repo.Create(context.Background(), domain.CreateGroupInput{Bundle: storedGroupFixture(t, asset), ActorID: author, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	return stored
}

func TestGroupUpdateReordersAndPreservesStableBlankTargets(t *testing.T) {
	ctx := context.Background()
	tx, author, repo := groupTransaction(t)
	before := createStoredFixture(t, tx, author, repo)
	bundle := before.Bundle
	bundle.Group.Title = "Bài đọc đã sửa"
	bundle.Group.Members[0], bundle.Group.Members[1] = bundle.Group.Members[1], bundle.Group.Members[0]
	bundle.Questions[1].Input.Blanks[0].AcceptedAnswers = []string{"đã sửa"}
	updated, err := repo.Update(ctx, domain.UpdateGroupInput{GroupMutation: groupMutation(before, author), Bundle: bundle})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 2 || updated.Bundle.Group.Members[0].QuestionID != bundle.Questions[1].ID || updated.Bundle.Questions[0].Input.Blanks[0].AcceptedAnswers[0] != "đã sửa" {
		t.Fatalf("lost reordered edit: %+v", updated)
	}
	if err := updated.Bundle.Validate(); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Update(ctx, domain.UpdateGroupInput{GroupMutation: groupMutation(before, author), Bundle: bundle}); !errors.Is(err, domain.ErrStaleWrite) {
		t.Fatalf("stale group write accepted: %v", err)
	}
	if _, err := tx.Exec(ctx, `SET CONSTRAINTS ALL IMMEDIATE`); err != nil {
		t.Fatalf("stable gap reference failed commit checks: %v", err)
	}
}

func TestGroupUpdateReplacesMembersAndOptionPolicyAtomically(t *testing.T) {
	ctx := context.Background()
	tx, author, repo := groupTransaction(t)
	before := createStoredFixture(t, tx, author, repo)
	kept := before.Bundle.Questions[0].ID
	added := groupIdentity(t)
	bundle := domain.GroupBundle{
		Group: domain.QuestionGroup{ID: before.Bundle.Group.ID, Title: "Replaced", Members: []domain.GroupMember{{QuestionID: kept, OptionOrder: "shuffle"}, {QuestionID: added, OptionOrder: "fixed"}}},
		Questions: []domain.GroupQuestion{
			{ID: kept, Input: questions.Input{Type: questions.ShortAnswer, Prompt: "New response", Points: "2"}},
			{ID: added, Input: questions.Input{Type: questions.SingleChoice, Prompt: "New choice", Points: "1", Options: []questions.OptionInput{{Text: "A", IsCorrect: true}, {Text: "B"}}}},
		},
	}
	updated, err := repo.Update(ctx, domain.UpdateGroupInput{GroupMutation: groupMutation(before, author), Bundle: bundle})
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Bundle.Questions) != 2 || updated.Bundle.Questions[0].Input.Type != questions.ShortAnswer || updated.Bundle.Group.Members[1].OptionOrder != "fixed" {
		t.Fatalf("membership/type transition failed: %+v", updated)
	}
	var removed int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.questions WHERE id=$1`, before.Bundle.Questions[1].ID).Scan(&removed); err != nil || removed != 0 {
		t.Fatalf("removed member leaked into bank: %d, %v", removed, err)
	}
	refs, err := mediarepo.GroupReferences(ctx, tx, before.Bundle.Group.Recordings[0].AssetID)
	if err != nil || len(refs) != 0 {
		t.Fatalf("removed materials left stale media bindings: %+v, %v", refs, err)
	}
	bundle = updated.Bundle
	bundle.Group.Members[0].OptionOrder = "fixed"
	bundle.Questions[0].Input = questions.Input{Type: questions.SingleChoice, Prompt: "Back to choice", Points: "2", Options: []questions.OptionInput{{Text: "A", IsCorrect: true}, {Text: "B"}}}
	if _, err := repo.Update(ctx, domain.UpdateGroupInput{GroupMutation: groupMutation(updated, author), Bundle: bundle}); err != nil {
		t.Fatalf("fixed policy and type were not changed together: %v", err)
	}
}

func TestGroupUpdateRollsBackAfterLateMediaFailure(t *testing.T) {
	ctx := context.Background()
	tx, author, repo := groupTransaction(t)
	before := createStoredFixture(t, tx, author, repo)
	original, err := json.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	image := storedGroupAsset(t, tx, author, "image")
	bundle := storedGroupFixture(t, image)
	bundle.Group.ID = before.Bundle.Group.ID
	if _, err := repo.Update(ctx, domain.UpdateGroupInput{GroupMutation: groupMutation(before, author), Bundle: bundle}); err == nil {
		t.Fatal("image accepted as shared audio")
	}
	after, err := repo.Get(ctx, before.Bundle.Group.ID)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := json.Marshal(after)
	if err != nil || string(actual) != string(original) {
		t.Fatalf("failed edit changed graph or revision: %v", err)
	}
	var events int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE entity_id=$1 AND action='question_group.updated'`, before.Bundle.Group.ID).Scan(&events); err != nil || events != 0 {
		t.Fatalf("failed update leaked audit: %d, %v", events, err)
	}
}

func TestGroupArchiveRestoreAndDeleteRequireCurrentRevision(t *testing.T) {
	ctx := context.Background()
	tx, author, repo := groupTransaction(t)
	stored := createStoredFixture(t, tx, author, repo)
	asset := stored.Bundle.Group.Recordings[0].AssetID
	if err := repo.Delete(ctx, groupMutation(stored, author)); !errors.Is(err, domain.ErrNotArchived) {
		t.Fatalf("active group deleted: %v", err)
	}
	archived, err := repo.SetArchived(ctx, groupMutation(stored, author), true)
	if err != nil || archived.ArchivedAt == nil || archived.Revision != 2 {
		t.Fatalf("archive: %+v, %v", archived, err)
	}
	if _, err := repo.Update(ctx, domain.UpdateGroupInput{GroupMutation: groupMutation(archived, author), Bundle: stored.Bundle}); err == nil {
		t.Fatal("archived context accepted edits")
	}
	if err := repo.Delete(ctx, groupMutation(stored, author)); !errors.Is(err, domain.ErrStaleWrite) {
		t.Fatalf("stale delete accepted: %v", err)
	}
	restored, err := repo.SetArchived(ctx, groupMutation(archived, author), false)
	if err != nil || restored.ArchivedAt != nil || restored.Revision != 3 {
		t.Fatalf("restore: %+v, %v", restored, err)
	}
	archived, err = repo.SetArchived(ctx, groupMutation(restored, author), true)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(ctx, groupMutation(archived, author)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, stored.Bundle.Group.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted group still readable: %v", err)
	}
	var members, events int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.questions WHERE context_group_id=$1`, stored.Bundle.Group.ID).Scan(&members); err != nil || members != 0 {
		t.Fatalf("orphaned members: %d, %v", members, err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE entity_id=$1`, stored.Bundle.Group.ID).Scan(&events); err != nil || events != 5 {
		t.Fatalf("audit history lost: %d, %v", events, err)
	}
	if err := mediarepo.NewPostgres(db.NewContext(tx)).SoftDelete(ctx, mediadomain.DeleteInput{ID: asset, ActorID: author, Now: time.Now()}); err != nil {
		t.Fatalf("deleted graph retained media references: %v", err)
	}
	if _, err := tx.Exec(ctx, `SET CONSTRAINTS ALL IMMEDIATE`); err != nil {
		t.Fatal(err)
	}
}

func TestGroupUpdateChecksParentDraftAndCannotArchiveOwnedGroup(t *testing.T) {
	ctx := context.Background()
	tx, author, repo := groupTransaction(t)
	var testID, sectionID string
	var updated time.Time
	if err := tx.QueryRow(ctx, `INSERT INTO app.tests (title,created_by) VALUES ('Draft',$1) RETURNING id::text,updated_at`, author).Scan(&testID, &updated); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO app.test_sections (test_id,ordinal,title) VALUES ($1,0,'Part') RETURNING id::text`, testID).Scan(&sectionID); err != nil {
		t.Fatal(err)
	}
	bundle := storedGroupFixture(t, "")
	stored, err := repo.Create(ctx, domain.CreateGroupInput{Bundle: bundle, OwnerSectionID: &sectionID, ExpectedTestUpdatedAt: updated, ActorID: author, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT updated_at FROM app.tests WHERE id=$1`, testID).Scan(&updated); err != nil {
		t.Fatal(err)
	}
	mutation := groupMutation(stored, author)
	mutation.ExpectedTestUpdatedAt = updated.Add(-time.Second)
	bundle.Group.Title = "Edited group"
	if _, err := repo.Update(ctx, domain.UpdateGroupInput{GroupMutation: mutation, Bundle: bundle}); !errors.Is(err, domain.ErrStaleWrite) {
		t.Fatalf("stale parent accepted edit: %v", err)
	}
	mutation.ExpectedTestUpdatedAt = updated
	if _, err := repo.SetArchived(ctx, mutation, true); err == nil {
		t.Fatal("test-owned group archived independently")
	}
	if err := repo.Delete(ctx, mutation); err == nil {
		t.Fatal("test-owned group deleted through bank lifecycle")
	}
	if _, err := repo.Update(ctx, domain.UpdateGroupInput{GroupMutation: mutation, Bundle: bundle}); err != nil {
		t.Fatal(err)
	}
	mutation.ExpectedRevision++
	if _, err := tx.Exec(ctx, `UPDATE app.tests SET status='archived' WHERE id=$1`, testID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT updated_at FROM app.tests WHERE id=$1`, testID).Scan(&mutation.ExpectedTestUpdatedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Update(ctx, domain.UpdateGroupInput{GroupMutation: mutation, Bundle: bundle}); err == nil {
		t.Fatal("archived parent accepted group edit")
	}
}

func TestGroupCopyAndSectionRemovalKeepIndependentContext(t *testing.T) {
	ctx := context.Background()
	tx, author, repo := groupTransaction(t)
	source := createStoredFixture(t, tx, author, repo)
	var testID, sectionID string
	var updated time.Time
	if err := tx.QueryRow(ctx, `INSERT INTO app.tests (title,created_by) VALUES ('Destination',$1) RETURNING id::text,updated_at`, author).Scan(&testID, &updated); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO app.test_sections (test_id,ordinal,title) VALUES ($1,0,'Part') RETURNING id::text`, testID).Scan(&sectionID); err != nil {
		t.Fatal(err)
	}
	in := domain.CopyGroupInput{SourceID: source.Bundle.Group.ID, ExpectedSourceRevision: 0, OwnerSectionID: &sectionID, ExpectedTestUpdatedAt: updated, ActorID: author, Now: time.Now()}
	if _, err := repo.Copy(ctx, in); !errors.Is(err, domain.ErrStaleWrite) {
		t.Fatalf("stale source copied: %v", err)
	}
	in.ExpectedSourceRevision = source.Revision
	first, err := repo.Copy(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT updated_at FROM app.tests WHERE id=$1`, testID).Scan(&in.ExpectedTestUpdatedAt); err != nil {
		t.Fatal(err)
	}
	second, err := repo.Copy(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	mutation := groupMutation(first, author)
	if err := tx.QueryRow(ctx, `SELECT updated_at FROM app.tests WHERE id=$1`, testID).Scan(&mutation.ExpectedTestUpdatedAt); err != nil {
		t.Fatal(err)
	}
	if err := repo.RemoveFromSection(ctx, mutation); err != nil {
		t.Fatal(err)
	}
	var onlyGroup string
	var ordinal, count int
	if err := tx.QueryRow(ctx, `SELECT group_id::text,ordinal,count(*) OVER () FROM app.test_section_units WHERE test_section_id=$1`, sectionID).Scan(&onlyGroup, &ordinal, &count); err != nil || onlyGroup != second.Bundle.Group.ID || ordinal != 0 || count != 1 {
		t.Fatalf("removal failed to compact remaining units: %s/%d/%d, %v", onlyGroup, ordinal, count, err)
	}
	archived, err := repo.SetArchived(ctx, groupMutation(source, author), true)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(ctx, groupMutation(archived, author)); err != nil {
		t.Fatal(err)
	}
	remaining, err := repo.Get(ctx, second.Bundle.Group.ID)
	if err != nil || len(remaining.Bundle.Questions) != 2 || remaining.Bundle.Group.Recordings[0].AssetID != source.Bundle.Group.Recordings[0].AssetID {
		t.Fatalf("source lifecycle damaged independent section copy: %+v, %v", remaining, err)
	}
	if remaining.Bundle.Group.ID == source.Bundle.Group.ID || remaining.Bundle.Questions[0].ID == source.Bundle.Questions[0].ID {
		t.Fatal("copy reused editable identities")
	}
	if _, err := tx.Exec(ctx, `SET CONSTRAINTS ALL IMMEDIATE`); err != nil {
		t.Fatal(err)
	}
}
