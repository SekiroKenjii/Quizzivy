//go:build integration

package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"quizzivy/internal/core/adapters"
	mediarepo "quizzivy/internal/modules/media/repositories"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
)

func TestSectionGroupsAndDuplicatesBelongToTheTestsOwner(t *testing.T) {
	ctx := context.Background()
	tx, owner, groups := groupTransaction(t)
	var other string
	if err := tx.QueryRow(ctx, `INSERT INTO app.users (email, full_name, role) VALUES ($1, 'Other author', 'admin') RETURNING id::text`, uuid.NewString()+"@example.test").Scan(&other); err != nil {
		t.Fatal(err)
	}
	testID, section, updated := snapshotDraft(t, tx, owner)
	asset := storedGroupAsset(t, tx, owner, "audio")
	sectionGroup, err := groups.Create(ctx, domain.CreateGroupInput{Bundle: storedGroupFixture(t, asset), OwnerSectionID: &section, ExpectedTestUpdatedAt: updated, ActorID: other, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	bankGroup, err := groups.Create(ctx, domain.CreateGroupInput{Bundle: storedGroupFixture(t, asset), ActorID: other, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	ownersOf := func(groupID string) (string, []string) {
		t.Helper()
		var group string
		var members []string
		if err := tx.QueryRow(ctx, `SELECT owner_id::text, ARRAY(SELECT q.owner_id::text FROM app.questions q WHERE q.context_group_id = g.id)
			FROM app.question_groups g WHERE g.id = $1`, groupID).Scan(&group, &members); err != nil {
			t.Fatal(err)
		}
		return group, members
	}
	for name, c := range map[string]struct{ id, want string }{
		"a section group another author adds": {sectionGroup.Bundle.Group.ID, owner},
		"a bank group":                        {bankGroup.Bundle.Group.ID, other},
	} {
		group, members := ownersOf(c.id)
		if group != c.want || len(members) == 0 {
			t.Errorf("%s: owner %s with %d members, want %s", name, group, len(members), c.want)
		}
		for _, member := range members {
			if member != group {
				t.Errorf("%s: a member belongs to %s, not the group's owner %s", name, member, group)
			}
		}
	}

	repo := repositories.NewPostgres(db.NewContext(tx), adapters.GroupQuestions{}, mediarepo.NewPostgres(db.NewContext(tx))).WithGroupQuestions(adapters.GroupQuestions{})
	copy, err := repo.Duplicate(ctx, domain.DuplicateInput{ID: testID, ActorID: other, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	var copyOwner, copyAuthor, copiedGroup string
	if err := tx.QueryRow(ctx, `SELECT t.owner_id::text, t.created_by::text, g.id::text
		FROM app.tests t JOIN app.test_sections s ON s.test_id = t.id JOIN app.question_groups g ON g.owner_section_id = s.id
		WHERE t.id = $1`, copy.ID).Scan(&copyOwner, &copyAuthor, &copiedGroup); err != nil {
		t.Fatal(err)
	}
	if copyOwner != owner || copyAuthor != other {
		t.Errorf("the copy belongs to %s and was made by %s, want the source's owner %s and the actor %s", copyOwner, copyAuthor, owner, other)
	}
	group, members := ownersOf(copiedGroup)
	if group != owner {
		t.Errorf("the copied group belongs to %s, want %s", group, owner)
	}
	for _, member := range members {
		if member != owner {
			t.Errorf("a copied member belongs to %s, want %s", member, owner)
		}
	}
}

func TestARestoredDraftsQuestionsBelongToTheTestsOwner(t *testing.T) {
	pool := newPool(t)
	other := pubMakeAuthor(t, pool)
	owner := pubMakeAuthor(t, pool)
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `UPDATE app.questions SET owner_id = created_by WHERE created_by = $1`, other); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	b := newBuilder(t, pool, owner)
	draft := b.draft("Restore ownership", b.shortAnswer("Snapshot", "1.00"))
	if _, err := b.publish(draft.ID); err != nil {
		t.Fatal(err)
	}
	request := versionRequest(t, b, draft.ID, 1)
	request.ActorID = other
	restored, err := b.tests.Commands.CreateDraftFromVersion.Handle(context.Background(), command.CreateDraftFromVersion{Request: request})
	if err != nil {
		t.Fatal(err)
	}
	var questionOwner, questionAuthor string
	if err := pool.QueryRow(context.Background(), `SELECT owner_id::text, created_by::text FROM app.questions WHERE id = $1`, restored.Sections[0].QuestionIDs[0]).Scan(&questionOwner, &questionAuthor); err != nil {
		t.Fatal(err)
	}
	if questionOwner != owner || questionAuthor != other {
		t.Errorf("the restored question belongs to %s and was made by %s, want the test's owner %s and the actor %s", questionOwner, questionAuthor, owner, other)
	}
}
