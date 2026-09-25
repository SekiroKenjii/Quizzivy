//go:build integration

package application_test

import (
	"context"
	"quizzivy/internal/core/adapters"
	mediarepo "quizzivy/internal/modules/media/repositories"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"testing"
	"time"
)

func TestDuplicateKeepsMixedUnitOrderAndSurvivesSourceTestDeletion(t *testing.T) {
	ctx := context.Background()
	tx, author, groups := groupTransaction(t)
	testID, section, updated := snapshotDraft(t, tx, author)
	asset := storedGroupAsset(t, tx, author, "audio")
	source, err := groups.Create(ctx, domain.CreateGroupInput{Bundle: storedGroupFixture(t, asset), OwnerSectionID: &section, ExpectedTestUpdatedAt: updated, ActorID: author, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SET CONSTRAINTS app.test_section_units_ordinal_key DEFERRED`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE app.test_section_units SET ordinal=1-ordinal WHERE test_section_id=$1`, section); err != nil {
		t.Fatal(err)
	}
	media := mediarepo.NewPostgres(db.NewContext(tx))
	repo := repositories.NewPostgres(db.NewContext(tx), adapters.GroupQuestions{}, media).WithGroupQuestions(adapters.GroupQuestions{})
	copy, err := repo.Duplicate(ctx, domain.DuplicateInput{ID: testID, ActorID: author, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if copy.ID == testID || copy.CurrentVersion != 0 || copy.Status != domain.Draft {
		t.Fatalf("copy inherited source state: %+v", copy)
	}
	var groupID string
	if err := tx.QueryRow(ctx, `SELECT u.group_id::text FROM app.test_section_units u JOIN app.test_sections s ON s.id=u.test_section_id
		WHERE s.test_id=$1 AND u.ordinal=0`, copy.ID).Scan(&groupID); err != nil {
		t.Fatal(err)
	}
	copied, err := groups.Get(ctx, groupID)
	if err != nil {
		t.Fatal(err)
	}
	if groupID == source.Bundle.Group.ID || copied.Bundle.Questions[0].ID == source.Bundle.Questions[0].ID {
		t.Fatal("duplicate reused editable context")
	}
	if copied.Bundle.Group.Members[0].OptionOrder != "fixed" || *copied.Bundle.Group.Recordings[0].Policy.MaxPlays != 2 {
		t.Fatal("copy changed group policy")
	}
	if _, err := tx.Exec(ctx, `UPDATE app.tests SET status='archived' WHERE id=$1`, testID); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(ctx, domain.Request{ID: testID, ActorID: author}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := groups.Get(ctx, groupID); err != nil {
		t.Fatalf("source test deletion destroyed copied group: %v", err)
	}
	var orphans int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.questions WHERE context_group_id=$1`, source.Bundle.Group.ID).Scan(&orphans); err != nil || orphans != 0 {
		t.Fatalf("deleted source stranded owned children: %d, %v", orphans, err)
	}
	published, err := repo.Publish(ctx, domain.PublishRequest{TestID: copy.ID, ActorID: author}, time.Now(), domain.Publishing.Validate)
	if err != nil || published.QuestionCount != 3 || published.TotalPoints != "2.00" {
		t.Fatalf("copied context failed publication: %+v, %v", published, err)
	}
	var order []string
	if err := tx.QueryRow(ctx, `SELECT array_agg(q.type::text ORDER BY s.ordinal,q.ordinal) FROM app.test_version_questions q
		JOIN app.test_version_sections s ON s.id=q.test_version_section_id WHERE s.test_version_id=$1`, published.ID).Scan(&order); err != nil {
		t.Fatal(err)
	}
	if len(order) != 3 || order[0] != "single_choice" || order[1] != "fill_blank" || order[2] != "short_answer" {
		t.Fatalf("mixed unit order changed: %+v", order)
	}
	if _, err := tx.Exec(ctx, `SET CONSTRAINTS ALL IMMEDIATE`); err != nil {
		t.Fatal(err)
	}
}
