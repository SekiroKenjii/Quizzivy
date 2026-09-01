package publish_test

import (
	"context"
	"errors"
	"testing"

	"quizzivy/internal/questions"
	"quizzivy/internal/tests"
)

// The version history and the student preview both read only the frozen rows.
// They are tested here rather than in internal/tests because the only way to
// get a version is to publish one.

func TestListVersionsIsNewestFirstAndCountsFrozenRows(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	b := newBuilder(t, pool, author)
	svc := tests.NewService(tests.NewStore(pool))
	ctx := context.Background()

	first := b.shortAnswer("Câu một", "2.00")
	second := b.shortAnswer("Câu hai", "3.00")
	draft := b.draft("Lịch sử phiên bản", first, second)

	if _, err := b.publish(draft.ID); err != nil {
		t.Fatalf("publish v1: %v", err)
	}
	if _, err := b.publish(draft.ID); err != nil {
		t.Fatalf("publish v2: %v", err)
	}

	versions, err := svc.ListVersions(ctx, draft.ID)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("want 2 versions, got %d", len(versions))
	}
	// Newest first: the history is read top-down and the current one matters most.
	if versions[0].Version != 2 || versions[1].Version != 1 {
		t.Fatalf("want [2 1], got [%d %d]", versions[0].Version, versions[1].Version)
	}
	if versions[0].QuestionCount != 2 {
		t.Errorf("question count: want 2, got %d", versions[0].QuestionCount)
	}
	if versions[0].TotalPoints != "5.00" {
		t.Errorf("total points: want 5.00, got %s", versions[0].TotalPoints)
	}
	if versions[0].PublishedBy != "Giáo viên" {
		t.Errorf("published by: want the display name, got %q", versions[0].PublishedBy)
	}
}

func TestPreviewRendersTheFrozenVersionNotTheDraft(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	b := newBuilder(t, pool, author)
	svc := tests.NewService(tests.NewStore(pool))
	qsvc := questions.NewService(questions.NewStore(pool))
	ctx := context.Background()

	questionID := b.question(questions.Input{
		Type:   questions.SingleChoice,
		Prompt: "Bản đã phát hành",
		Points: "2.00",
		Options: []questions.OptionInput{
			{Text: "đúng", IsCorrect: true},
			{Text: "sai", IsCorrect: false},
		},
	})
	draft := b.draft("Xem trước", questionID)
	if _, err := b.publish(draft.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// The teacher rewrites the bank question after publishing.
	if _, err := qsvc.Update(ctx, questions.WriteRequest{
		ID:      questionID,
		ActorID: author,
		Input: questions.Input{
			Type:   questions.SingleChoice,
			Prompt: "Bản nháp đã sửa",
			Points: "9.00",
			Tags:   []string{},
			Options: []questions.OptionInput{
				{Text: "khác", IsCorrect: true},
				{Text: "nữa", IsCorrect: false},
			},
		},
	}); err != nil {
		t.Fatalf("edit the bank question: %v", err)
	}

	version, questionsOut, err := svc.Preview(ctx, draft.ID, 0)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if version != 1 {
		t.Errorf("version: want 1, got %d", version)
	}
	if len(questionsOut) != 1 {
		t.Fatalf("want 1 question, got %d", len(questionsOut))
	}
	// §7: the version holds its own snapshot, so a bank edit cannot reach a
	// student sitting the published test.
	if questionsOut[0].Prompt != "Bản đã phát hành" {
		t.Errorf("prompt: want the frozen text, got %q", questionsOut[0].Prompt)
	}
	if questionsOut[0].Points != "2.00" {
		t.Errorf("points: want the frozen 2.00, got %s", questionsOut[0].Points)
	}
	if len(questionsOut[0].Options) != 2 {
		t.Fatalf("want 2 options, got %d", len(questionsOut[0].Options))
	}
	if questionsOut[0].Options[0].Text != "đúng" {
		t.Errorf("option: want the frozen text, got %q", questionsOut[0].Options[0].Text)
	}
}

func TestPreviewOfAnUnpublishedTestSaysSoRatherThanReturningNothing(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	b := newBuilder(t, pool, author)
	svc := tests.NewService(tests.NewStore(pool))

	draft := b.draft("Chưa phát hành", b.shortAnswer("Câu một", "1.00"))

	// An empty list would render as a published test with no questions.
	if _, _, err := svc.Preview(context.Background(), draft.ID, 0); !errors.Is(err, tests.ErrNotPublished) {
		t.Fatalf("want ErrNotPublished, got %v", err)
	}
}

func TestPreviewPinsAnOlderVersion(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	b := newBuilder(t, pool, author)
	svc := tests.NewService(tests.NewStore(pool))
	ctx := context.Background()

	one := b.shortAnswer("Chỉ ở v1", "1.00")
	draft := b.draft("Hai phiên bản", one)
	if _, err := b.publish(draft.ID); err != nil {
		t.Fatalf("publish v1: %v", err)
	}

	two := b.shortAnswer("Thêm ở v2", "1.00")
	reread, err := svc.Get(ctx, draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Update(ctx, tests.Request{ID: draft.ID, ActorID: author}, tests.UpdateInput{
		ExpectedUpdatedAt: reread.UpdatedAt,
		SetSections:       true,
		Sections:          []tests.SectionInput{{Title: "Phần 1", QuestionIDs: []string{one, two}}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.publish(draft.ID); err != nil {
		t.Fatalf("publish v2: %v", err)
	}

	v1, questionsV1, err := svc.Preview(ctx, draft.ID, 1)
	if err != nil {
		t.Fatalf("preview v1: %v", err)
	}
	if v1 != 1 || len(questionsV1) != 1 {
		t.Fatalf("v1: want version 1 with 1 question, got %d with %d", v1, len(questionsV1))
	}

	v2, questionsV2, err := svc.Preview(ctx, draft.ID, 0)
	if err != nil {
		t.Fatalf("preview current: %v", err)
	}
	if v2 != 2 || len(questionsV2) != 2 {
		t.Fatalf("current: want version 2 with 2 questions, got %d with %d", v2, len(questionsV2))
	}
}
