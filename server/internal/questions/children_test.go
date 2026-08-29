package questions_test

import (
	"context"
	"testing"

	"quizzivy/internal/questions"
)

func TestListedPageCarriesEachQuestionsOwnChildren(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)
	ctx := context.Background()

	choice, err := svc.Create(ctx, questions.WriteRequest{
		Input: questions.Input{
			Type: questions.SingleChoice, Prompt: "Trộn con — chọn", Points: "1.00", Tags: []string{},
			Options: []questions.OptionInput{{Text: "Đúng", IsCorrect: true}, {Text: "Sai", IsCorrect: false}},
		}, ActorID: author})
	if err != nil {
		t.Fatal(err)
	}
	blank, err := svc.Create(ctx, questions.WriteRequest{
		Input: questions.Input{
			Type: questions.FillBlank, Prompt: "Trộn con — điền {{1}}", Points: "1.00", Tags: []string{},
			Blanks: []questions.BlankInput{{Ordinal: 1, AcceptedAnswers: []string{"a", "b"}}},
		}, ActorID: author})
	if err != nil {
		t.Fatal(err)
	}
	plain, err := svc.Create(ctx, questions.WriteRequest{
		Input: questions.Input{
			Type: questions.ShortAnswer, Prompt: "Trộn con — viết", Points: "1.00", Tags: []string{},
		}, ActorID: author})
	if err != nil {
		t.Fatal(err)
	}

	listed, _, err := svc.List(ctx, questions.ListInput{Query: "Tron con", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]questions.Question{}
	for _, q := range listed {
		byID[q.ID] = q
	}

	if got := byID[choice.ID]; len(got.Options) != 2 || len(got.Blanks) != 0 {
		t.Errorf("choice question: %d options %d blanks, want 2 and 0", len(got.Options), len(got.Blanks))
	}
	if got := byID[blank.ID]; len(got.Blanks) != 1 || len(got.Options) != 0 {
		t.Errorf("fill_blank: %d blanks %d options, want 1 and 0", len(got.Blanks), len(got.Options))
	} else if len(got.Blanks[0].AcceptedAnswers) != 2 {
		t.Errorf("fill_blank answers: %v, want 2", got.Blanks[0].AcceptedAnswers)
	}
	if got := byID[plain.ID]; len(got.Options) != 0 || len(got.Blanks) != 0 {
		t.Errorf("short_answer got children it should not have: %d/%d", len(got.Options), len(got.Blanks))
	}
	if byID[plain.ID].Options == nil || byID[plain.ID].Blanks == nil {
		t.Error("empty children came back nil; the contract's arrays are never null")
	}
}
