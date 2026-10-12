//go:build integration

package application_test

import (
	"context"
	"fmt"
	"quizzivy/internal/core/adapters"
	mediarepo "quizzivy/internal/modules/media/repositories"
	questionscommand "quizzivy/internal/modules/questions/application/command"
	questionsdomain "quizzivy/internal/modules/questions/domain"
	questionsrepo "quizzivy/internal/modules/questions/repositories"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/application/query"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"slices"
	"testing"
	"time"
)

func (b *builder) skilled(prompt string, skill questionsdomain.Skill) string {
	return b.question(questionsdomain.Input{Type: questionsdomain.ShortAnswer, Prompt: prompt, Points: "1.00", Skill: &skill})
}

func (b *builder) versionsOf(testID string) []domain.Version {
	b.t.Helper()
	versions, err := b.tests.Queries.ListVersions.Handle(context.Background(), query.ListVersions{TestID: testID, Scope: everyone})
	if err != nil {
		b.t.Fatalf("list versions: %v", err)
	}
	return versions
}

func (b *builder) draftInSections(title string, parts ...[]string) domain.Test {
	b.t.Helper()
	ctx := context.Background()
	created, err := b.tests.Commands.Create.Handle(ctx, command.Create{Request: domain.Request{ActorID: b.author, Scope: access.Scope{UserID: b.author}}, Title: title})
	if err != nil {
		b.t.Fatal(err)
	}
	sections := make([]domain.SectionInput, len(parts))
	for i, ids := range parts {
		sections[i] = domain.SectionInput{Title: fmt.Sprintf("Phần %d", i+1), QuestionIDs: ids}
	}
	saved, err := b.tests.Commands.Update.Handle(ctx, command.Update{Request: domain.Request{ID: created.ID, ActorID: b.author, Scope: access.Scope{UserID: b.author}}, Input: domain.UpdateInput{
		ExpectedUpdatedAt: created.UpdatedAt, SetSections: true, Sections: sections,
	}})
	if err != nil {
		b.t.Fatal(err)
	}
	return saved
}

func (b *builder) draftOf(testID string) domain.Test {
	b.t.Helper()
	repo := repositories.NewPostgres(db.NewContext(b.pool), questionsrepo.NewPostgres(db.NewContext(b.pool)), mediarepo.NewPostgres(db.NewContext(b.pool)))
	current, err := repo.Get(context.Background(), everyone, testID)
	if err != nil {
		b.t.Fatalf("read the draft: %v", err)
	}
	return current
}

func TestAVersionFreezesTheSortedDistinctSkillsOfItsQuestions(t *testing.T) {
	b := newSnapshotFixture(t).builder

	draft := b.draftInSections("Kỹ năng đã đóng băng",
		[]string{b.skilled("Đọc một", "reading"), b.skilled("Ngữ pháp", "grammar")},
		[]string{b.skilled("Đọc hai", "reading"), b.skilled("Nghe", "listening"), b.shortAnswer("Chưa gán kỹ năng", "1.00")},
	)
	published, err := b.publish(draft.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	want := []string{"grammar", "listening", "reading"}
	if !slices.Equal(published.Skills, want) {
		t.Errorf("the publish answered skills %v, want %v", published.Skills, want)
	}
	versions := b.versionsOf(draft.ID)
	if len(versions) != 1 || !slices.Equal(versions[0].Skills, want) {
		t.Errorf("the history answered %+v, want one version with skills %v", versions, want)
	}
}

func TestAVersionWhoseQuestionsCarryNoSkillAnswersAnEmptyListNotNil(t *testing.T) {
	b := newSnapshotFixture(t).builder

	draft := b.draft("Không có kỹ năng", b.shortAnswer("Câu một", "1.00"), b.shortAnswer("Câu hai", "1.00"))
	published, err := b.publish(draft.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	if published.Skills == nil || len(published.Skills) != 0 {
		t.Errorf("the publish answered skills %#v, want an empty non-nil list", published.Skills)
	}
	versions := b.versionsOf(draft.ID)
	if len(versions) != 1 || versions[0].Skills == nil || len(versions[0].Skills) != 0 {
		t.Errorf("the history answered %+v, want an empty non-nil list", versions)
	}
}

func TestEditingTheDraftAndTheBankAfterPublishLeavesTheVersionsSkillsUnchanged(t *testing.T) {
	fixture := newSnapshotFixture(t)
	b := fixture.builder
	ctx := context.Background()

	grammar := b.skilled("Ngữ pháp", "grammar")
	reading := b.skilled("Đọc hiểu", "reading")
	draft := b.draft("Kỹ năng sau khi xuất bản", grammar, reading)
	first, err := b.publish(draft.ID)
	if err != nil {
		t.Fatalf("publish v1: %v", err)
	}
	frozen := []string{"grammar", "reading"}
	if !slices.Equal(first.Skills, frozen) {
		t.Fatalf("version 1 skills %v, want %v", first.Skills, frozen)
	}

	speaking := questionsdomain.Skill("speaking")
	if _, err := b.qsvc.Commands.Update.Handle(ctx, questionscommand.Update{Request: questionsdomain.WriteRequest{
		ID: grammar, ActorID: fixture.author,
		Input: questionsdomain.Input{Type: questionsdomain.ShortAnswer, Prompt: "Ngữ pháp", Points: "1.00", Tags: []string{}, Skill: &speaking},
	}}); err != nil {
		t.Fatalf("editing the bank question: %v", err)
	}
	listening := b.skilled("Nghe", "listening")
	if _, err := b.tests.Commands.Update.Handle(ctx, command.Update{Request: reqFor(draft.ID, fixture.author), Input: domain.UpdateInput{
		ExpectedUpdatedAt: b.draftOf(draft.ID).UpdatedAt, SetSections: true,
		Sections: []domain.SectionInput{{Title: "Phần 1", QuestionIDs: []string{grammar, listening}}},
	}}); err != nil {
		t.Fatalf("editing the draft: %v", err)
	}

	if moved := b.draftOf(draft.ID); !slices.Equal(moved.Skills, []string{"listening", "speaking"}) {
		t.Fatalf("the draft's skills are %v; the edits did not reach it, so the test below proves nothing", moved.Skills)
	}
	versions := b.versionsOf(draft.ID)
	if len(versions) != 1 || !slices.Equal(versions[0].Skills, frozen) {
		t.Fatalf("after the edits the history answered %+v, want version 1 with skills %v", versions, frozen)
	}

	second, err := b.publish(draft.ID)
	if err != nil {
		t.Fatalf("publish v2: %v", err)
	}
	if !slices.Equal(second.Skills, []string{"listening", "speaking"}) {
		t.Errorf("version 2 skills %v, want the draft's at its publish", second.Skills)
	}
	versions = b.versionsOf(draft.ID)
	if len(versions) != 2 || versions[0].Version != 2 || versions[1].Version != 1 {
		t.Fatalf("history %+v, want versions 2 and 1", versions)
	}
	if !slices.Equal(versions[1].Skills, frozen) || !slices.Equal(versions[0].Skills, []string{"listening", "speaking"}) {
		t.Errorf("the history answered skills %v for version 2 and %v for version 1", versions[0].Skills, versions[1].Skills)
	}
}

func TestAVersionCountsTheSkillsOfAGroupsMembers(t *testing.T) {
	ctx := context.Background()
	tx, author, groups := groupTransaction(t)
	testID, section, updated := snapshotDraft(t, tx, author)
	bundle := storedGroupFixture(t, "")
	bundle.Questions[0].Input.Skill = groupValue(questionsdomain.Skill("grammar"))
	bundle.Questions[1].Input.Skill = groupValue(questionsdomain.Skill("vocabulary"))
	if _, err := groups.Create(ctx, domain.CreateGroupInput{Bundle: bundle, OwnerSectionID: &section, ExpectedTestUpdatedAt: updated, ActorID: author, Now: time.Now(), Scope: access.Scope{UserID: author}, Grants: bothKeys}); err != nil {
		t.Fatal(err)
	}

	media := mediarepo.NewPostgres(db.NewContext(tx))
	repo := repositories.NewPostgres(db.NewContext(tx), adapters.GroupQuestions{}, media).WithGroupQuestions(adapters.GroupQuestions{})
	published, err := repo.Publish(ctx, domain.PublishRequest{TestID: testID, ActorID: author, Scope: access.Scope{UserID: author}}, time.Now(), domain.Publishing.Validate)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"grammar", "vocabulary"}
	if !slices.Equal(published.Skills, want) {
		t.Errorf("the publish answered skills %v, want the members' %v", published.Skills, want)
	}
	versions, err := repo.ListVersions(ctx, everyone, testID)
	if err != nil || len(versions) != 1 || !slices.Equal(versions[0].Skills, want) {
		t.Errorf("the history answered %+v, %v; want one version with skills %v", versions, err, want)
	}
}
