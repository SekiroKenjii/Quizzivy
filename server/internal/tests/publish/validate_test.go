package publish_test

import (
	"context"
	"errors"
	"testing"

	"quizzivy/internal/questions"
	"quizzivy/internal/tests"
	"quizzivy/internal/tests/publish"
)

// violations runs a publish expected to fail and returns what it reported.
func violations(t *testing.T, b *builder, testID string) []publish.Violation {
	t.Helper()
	_, err := b.publish(testID)
	var invalid *publish.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("publish returned %v, want a ValidationError", err)
	}
	return invalid.Violations
}

// findRule returns the violation for a rule, so each case asserts the rule AND
// the anchor the builder needs to mark it inline.
func findRule(t *testing.T, got []publish.Violation, rule publish.Rule) publish.Violation {
	t.Helper()
	for _, v := range got {
		if v.Rule == rule {
			return v
		}
	}
	t.Fatalf("no %s violation in %+v", rule, got)
	return publish.Violation{}
}

func TestPublishRejectsAChoiceQuestionWithNoCorrectOption(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	b := newBuilder(t, pool, author)

	q := b.question(questions.Input{
		Type: questions.SingleChoice, Prompt: "Không có đáp án đúng", Points: "1.00",
		Options: []questions.OptionInput{{Text: "A", IsCorrect: true}, {Text: "B", IsCorrect: false}},
	})
	draft := b.draft("Đề thiếu đáp án", q)
	if _, err := pool.Exec(context.Background(),
		`UPDATE app.question_options SET is_correct = false WHERE question_id = $1`, q); err != nil {
		t.Fatal(err)
	}

	v := findRule(t, violations(t, b, draft.ID), publish.ChoiceHasCorrectOption)
	if v.QuestionID != q {
		t.Errorf("violation names question %q, want %q", v.QuestionID, q)
	}
	if v.SectionID == "" {
		t.Error("violation has no sectionId; the builder cannot locate it")
	}
}

func TestPublishRejectsABlankWithNoAcceptedAnswer(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	b := newBuilder(t, pool, author)

	q := b.question(questions.Input{
		Type: questions.FillBlank, Prompt: "Điền {{1}}", Points: "1.00",
		Blanks: []questions.BlankInput{{Ordinal: 1, AcceptedAnswers: []string{"x"}}},
	})
	draft := b.draft("Đề thiếu đáp án chỗ trống", q)
	if _, err := pool.Exec(context.Background(),
		`DELETE FROM app.question_blank_answers a
		  USING app.question_blanks b
		  WHERE a.blank_id = b.id AND b.question_id = $1`, q); err != nil {
		t.Fatal(err)
	}

	v := findRule(t, violations(t, b, draft.ID), publish.BlankHasAcceptedAnswer)
	if v.QuestionID != q {
		t.Errorf("violation names question %q, want %q", v.QuestionID, q)
	}
}

// §8 implies this without stating it: a prompt with {{3}} and only two blanks
// is unrenderable.
func TestPublishRejectsPlaceholdersThatDoNotMatchTheBlanks(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	b := newBuilder(t, pool, author)

	q := b.question(questions.Input{
		Type: questions.FillBlank, Prompt: "Điền {{1}} và {{2}}", Points: "1.00",
		Blanks: []questions.BlankInput{
			{Ordinal: 1, AcceptedAnswers: []string{"a"}},
			{Ordinal: 2, AcceptedAnswers: []string{"b"}},
		},
	})
	draft := b.draft("Đề lệch chỗ trống", q)
	// A third placeholder appears in the prompt with no blank behind it.
	if _, err := pool.Exec(context.Background(),
		`UPDATE app.questions SET prompt = 'Điền {{1}}, {{2}} và {{3}}' WHERE id = $1`, q); err != nil {
		t.Fatal(err)
	}

	v := findRule(t, violations(t, b, draft.ID), publish.BlankPlaceholdersMatch)
	if v.QuestionID != q {
		t.Errorf("violation names question %q, want %q", v.QuestionID, q)
	}
}

func TestPublishRejectsAnEmptySection(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	b := newBuilder(t, pool, author)
	ctx := context.Background()

	created, err := b.tests.Create(ctx, tests.Request{ActorID: author}, "Đề có phần rỗng", nil)
	if err != nil {
		t.Fatal(err)
	}
	q := b.shortAnswer("Câu duy nhất", "1.00")
	saved, err := b.tests.Update(ctx, tests.Request{ID: created.ID, ActorID: author},
		tests.UpdateInput{
			ExpectedUpdatedAt: created.UpdatedAt,
			SetSections:       true,
			Sections: []tests.SectionInput{
				{Title: "Phần có câu", QuestionIDs: []string{q}},
				{Title: "Phần rỗng", QuestionIDs: []string{}},
			},
		})
	if err != nil {
		t.Fatal(err)
	}

	v := findRule(t, violations(t, b, saved.ID), publish.SectionNotEmpty)
	if v.SectionID != saved.Sections[1].ID {
		t.Errorf("violation names section %q, want the empty one %q", v.SectionID, saved.Sections[1].ID)
	}
	if v.QuestionID != "" {
		t.Errorf("an empty-section violation named question %q; there is none", v.QuestionID)
	}
}

func TestPublishReportsEveryProblemAtOnce(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	b := newBuilder(t, pool, author)
	ctx := context.Background()

	bad := b.question(questions.Input{
		Type: questions.SingleChoice, Prompt: "Sai", Points: "1.00",
		Options: []questions.OptionInput{{Text: "A", IsCorrect: true}, {Text: "B", IsCorrect: false}},
	})
	alsoBad := b.question(questions.Input{
		Type: questions.FillBlank, Prompt: "Điền {{1}}", Points: "1.00",
		Blanks: []questions.BlankInput{{Ordinal: 1, AcceptedAnswers: []string{"x"}}},
	})
	created, err := b.tests.Create(ctx, tests.Request{ActorID: author}, "Đề nhiều lỗi", nil)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := b.tests.Update(ctx, tests.Request{ID: created.ID, ActorID: author},
		tests.UpdateInput{
			ExpectedUpdatedAt: created.UpdatedAt,
			SetSections:       true,
			Sections: []tests.SectionInput{
				{Title: "Phần 1", QuestionIDs: []string{bad, alsoBad}},
				{Title: "Phần rỗng", QuestionIDs: []string{}},
			},
		})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := pool.Exec(ctx,
		`UPDATE app.question_options SET is_correct = false WHERE question_id = $1`, bad); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`DELETE FROM app.question_blank_answers a USING app.question_blanks b
		  WHERE a.blank_id = b.id AND b.question_id = $1`, alsoBad); err != nil {
		t.Fatal(err)
	}

	got := violations(t, b, saved.ID)
	for _, rule := range []publish.Rule{
		publish.ChoiceHasCorrectOption,
		publish.BlankHasAcceptedAnswer,
		publish.SectionNotEmpty,
	} {
		findRule(t, got, rule)
	}
	if len(got) < 3 {
		t.Errorf("got %d violations, want at least 3 -- publishing must not stop at the first", len(got))
	}
}

func TestPublishingATestWithNoSectionsIsRefused(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	b := newBuilder(t, pool, author)

	created, err := b.tests.Create(context.Background(), tests.Request{ActorID: author}, "Đề rỗng", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.publish(created.ID); !errors.Is(err, publish.ErrNoContent) {
		t.Errorf("got %v, want ErrNoContent", err)
	}
}
