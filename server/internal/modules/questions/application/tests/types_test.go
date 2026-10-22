//go:build integration

package application_test

import (
	"context"
	"testing"

	"quizzivy/internal/modules/questions/application"
	"quizzivy/internal/modules/questions/application/command"
	"quizzivy/internal/modules/questions/application/query"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/shared/access"
)

// T-R6.3: create, update, reorder and duplicate for each of R6's question
// shapes, read back through the bank.

func ptr[T any](v T) *T { return &v }

func duplicate(t *testing.T, svc *application.Application, author, id string) domain.Question {
	t.Helper()
	q, err := svc.Commands.Duplicate.Handle(context.Background(), command.Duplicate{Request: domain.WriteRequest{ID: id, ActorID: author}})
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	return q
}

func optionInputs(q domain.Question) []domain.OptionInput {
	out := make([]domain.OptionInput, len(q.Options))
	for i, o := range q.Options {
		out[i] = domain.OptionInput{ID: ptr(o.ID), Text: o.Text, IsCorrect: o.IsCorrect}
	}
	return out
}

func TestATFNGQuestionStoresItsFixedTexts(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)

	in := domain.Input{Tags: []string{}, Type: domain.TFNG, Prompt: "The writer was born in Hue.", Points: "1",
		Options: []domain.OptionInput{{Text: "Đúng"}, {Text: "Sai"}, {Text: "Không có", IsCorrect: true}}}
	created := createQuestion(t, svc, author, in)
	want := []string{"True", "False", "Not given"}
	for _, q := range []domain.Question{created, reread(t, svc, created.ID)} {
		if len(q.Options) != 3 {
			t.Fatalf("%d options, want 3", len(q.Options))
		}
		for i, o := range q.Options {
			if o.Text != want[i] || o.IsCorrect != (i == 2) || o.Content != nil {
				t.Errorf("option %d = %q correct %v content %s, want %q correct %v", i, o.Text, o.IsCorrect, o.Content, want[i], i == 2)
			}
		}
	}

	edit := in
	edit.Options = optionInputs(created)
	edit.Options[2].IsCorrect, edit.Options[0].IsCorrect = false, true
	updated := updateQuestion(t, svc, author, created.ID, edit)
	if !updated.Options[0].IsCorrect || updated.Options[2].IsCorrect || updated.Options[0].ID != created.Options[0].ID {
		t.Errorf("moving the key to True gave %+v", updated.Options)
	}

	copied := duplicate(t, svc, author, created.ID)
	if copied.Type != domain.TFNG || len(copied.Options) != 3 || !copied.Options[0].IsCorrect || copied.Options[0].ID == created.Options[0].ID {
		t.Errorf("the duplicate is %+v", copied.Options)
	}
}

func TestChooseNAndWordLimitsRoundTrip(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)

	choose := choiceInput("Which two are cities?", "Hanoi", "Hue", "Mekong", "Red")
	choose.Type = domain.MultipleChoice
	choose.Options[1].IsCorrect = true
	choose.SelectCount = ptr(2)
	created := createQuestion(t, svc, author, choose)
	if got := reread(t, svc, created.ID).SelectCount; got == nil || *got != 2 {
		t.Errorf("selectCount read back as %v, want 2", got)
	}
	if got := duplicate(t, svc, author, created.ID).SelectCount; got == nil || *got != 2 {
		t.Errorf("the duplicate's selectCount is %v, want 2", got)
	}
	plain := choose
	plain.SelectCount = nil
	if got := updateQuestion(t, svc, author, created.ID, plain).SelectCount; got != nil {
		t.Errorf("an update without selectCount kept %d, want a plain multiple choice", *got)
	}

	short := createQuestion(t, svc, author, domain.Input{Tags: []string{}, Type: domain.ShortAnswer, Prompt: "Name the river.", Points: "1", MaxWords: ptr(3)})
	if got := reread(t, svc, short.ID).MaxWords; got == nil || *got != 3 {
		t.Errorf("maxWords read back as %v, want 3", got)
	}
	if got := duplicate(t, svc, author, short.ID).MaxWords; got == nil || *got != 3 {
		t.Errorf("the duplicate's maxWords is %v, want 3", got)
	}
}

func TestAnEssaysRubricKeepsItsIdsThroughAReorder(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)

	in := domain.Input{Tags: []string{}, Type: domain.Essay, Prompt: "Write about your city.", Points: "9", SampleAnswer: ptr("My city is Hue."),
		Rubric: []domain.RubricCriterionInput{{Label: "Task response"}, {Label: "Coherence"}, {Label: "Grammar"}}}
	created := createQuestion(t, svc, author, in)
	if len(created.Rubric) != 3 || created.Rubric[2].Label != "Grammar" || created.SampleAnswer == nil {
		t.Fatalf("created %+v sample %v", created.Rubric, created.SampleAnswer)
	}

	// Grammar first, Coherence dropped, a new one at the end.
	edit := in
	edit.Rubric = []domain.RubricCriterionInput{
		{ID: ptr(created.Rubric[2].ID), Label: "Grammar"},
		{ID: ptr(created.Rubric[0].ID), Label: "Task achievement"},
		{Label: "Lexical resource"},
	}
	updated := reread(t, svc, updateQuestion(t, svc, author, created.ID, edit).ID)
	want := []string{"Grammar", "Task achievement", "Lexical resource"}
	for i, criterion := range updated.Rubric {
		if criterion.Label != want[i] || criterion.Ordinal != i {
			t.Errorf("criterion %d = %q at %d, want %q", i, criterion.Label, criterion.Ordinal, want[i])
		}
	}
	if len(updated.Rubric) != 3 || updated.Rubric[0].ID != created.Rubric[2].ID || updated.Rubric[1].ID != created.Rubric[0].ID {
		t.Errorf("the reorder did not keep the criteria's ids: %+v", updated.Rubric)
	}

	copied := duplicate(t, svc, author, created.ID)
	if len(copied.Rubric) != 3 || copied.Rubric[0].Label != "Grammar" || copied.Rubric[0].ID == updated.Rubric[0].ID {
		t.Errorf("the duplicate's rubric is %+v", copied.Rubric)
	}

	// An essay retyped as a short answer leaves no criteria behind.
	updateQuestion(t, svc, author, created.ID, domain.Input{Tags: []string{}, Type: domain.ShortAnswer, Prompt: "Name it.", Points: "1"})
	var left int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM app.question_rubric_criteria WHERE question_id = $1`, created.ID).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Errorf("%d criteria left on a short answer, want 0", left)
	}
}

func matchingInput() domain.Input {
	return domain.Input{Tags: []string{}, Type: domain.Matching, Prompt: "Match each paragraph to a heading.", Points: "2", MatchAllowReuse: ptr(false),
		Options: []domain.OptionInput{{Text: "A busy port"}, {Text: "A quiet village"}, {Text: "A royal past"}},
		MatchItems: []domain.MatchItemInput{
			{Text: "Paragraph 1", CorrectOption: 2},
			{Text: "Paragraph 2", CorrectOption: 0},
		}}
}

func TestAMatchingSaveRewritesItsItemsBeforeItDeletesAChoice(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)

	created := createQuestion(t, svc, author, matchingInput())
	got := reread(t, svc, created.ID)
	if got.MatchAllowReuse == nil || *got.MatchAllowReuse || len(got.MatchItems) != 2 {
		t.Fatalf("read back reuse %v items %+v", got.MatchAllowReuse, got.MatchItems)
	}
	if got.MatchItems[0].CorrectOptionID != got.Options[2].ID || got.MatchItems[0].CorrectOption != 2 || got.MatchItems[1].CorrectOption != 0 {
		t.Errorf("items keyed %+v, want paragraph 1 to choice 2 and paragraph 2 to choice 0", got.MatchItems)
	}

	// Drop the choice paragraph 1 names, add a new one, key paragraph 1 to it
	// and swap the two items: one save, under the NO ACTION key.
	edit := matchingInput()
	edit.Options = []domain.OptionInput{
		{ID: ptr(got.Options[0].ID), Text: "A busy port"},
		{ID: ptr(got.Options[1].ID), Text: "A quiet village"},
		{Text: "A new bridge"},
	}
	edit.MatchItems = []domain.MatchItemInput{
		{ID: ptr(got.MatchItems[1].ID), Text: "Paragraph 2", CorrectOption: 0},
		{ID: ptr(got.MatchItems[0].ID), Text: "Paragraph 1", CorrectOption: 2},
	}
	updateQuestion(t, svc, author, created.ID, edit)
	after := reread(t, svc, created.ID)
	if len(after.Options) != 3 || after.Options[2].Text != "A new bridge" || after.Options[2].ID == got.Options[2].ID {
		t.Fatalf("options after the save: %+v", after.Options)
	}
	if after.MatchItems[0].ID != got.MatchItems[1].ID || after.MatchItems[1].ID != got.MatchItems[0].ID {
		t.Errorf("the reorder did not keep the items' ids: %+v", after.MatchItems)
	}
	if after.MatchItems[1].CorrectOptionID != after.Options[2].ID || after.MatchItems[0].CorrectOptionID != after.Options[0].ID {
		t.Errorf("items keyed %+v against options %+v", after.MatchItems, after.Options)
	}

	copied := duplicate(t, svc, author, created.ID)
	if len(copied.MatchItems) != 2 || copied.MatchItems[1].CorrectOption != 2 ||
		copied.MatchItems[1].CorrectOptionID != copied.Options[2].ID || copied.Options[2].ID == after.Options[2].ID {
		t.Errorf("the duplicate is not keyed to its own choices: %+v %+v", copied.MatchItems, copied.Options)
	}

	// Retyped as a single choice, its items go before the choices they name.
	single := choiceInput("Which is a port?", "Hai Phong", "Sa Pa")
	updateQuestion(t, svc, author, created.ID, single)
	var left int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM app.question_match_items WHERE question_id = $1`, created.ID).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Errorf("%d items left on a single choice, want 0", left)
	}
	if q := reread(t, svc, created.ID); q.MatchAllowReuse != nil || q.MatchItems != nil {
		t.Errorf("a single choice reads back reuse %v and items %+v", q.MatchAllowReuse, q.MatchItems)
	}
}

func TestFacetsCountTheNewTypes(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)

	createQuestion(t, svc, author, domain.Input{Tags: []string{}, Type: domain.TFNG, Prompt: "x", Points: "1",
		Options: []domain.OptionInput{{Text: "a", IsCorrect: true}, {Text: "b"}, {Text: "c"}}})
	createQuestion(t, svc, author, matchingInput())
	createQuestion(t, svc, author, matchingInput())
	createQuestion(t, svc, author, domain.Input{Tags: []string{}, Type: domain.Essay, Prompt: "Write.", Points: "9"})

	facets, err := svc.Queries.Facets.Handle(context.Background(), query.Facets{Input: domain.ListInput{Scope: access.Scope{UserID: author}}})
	if err != nil {
		t.Fatal(err)
	}
	if facets.ByType[domain.TFNG] != 1 || facets.ByType[domain.Matching] != 2 || facets.ByType[domain.Essay] != 1 || facets.All != 4 {
		t.Errorf("facets = all %d %v, want tfng 1, matching 2, essay 1 of 4", facets.All, facets.ByType)
	}
}

// QC-R6Q-1: an empty sample answer on a type that has none used to reach the
// store's check and answer 500. It is now dropped before the write.
func TestAnEmptySampleAnswerOnATypeWithoutOneIsDropped(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)

	tfng := domain.Input{Tags: []string{}, Type: domain.TFNG, Prompt: "The port is busy.", Points: "1",
		Options: []domain.OptionInput{{Text: "True", IsCorrect: true}, {Text: "False"}, {Text: "Not given"}}}
	choice := domain.Input{Tags: []string{}, Type: domain.MultipleChoice, Prompt: "Pick one.", Points: "1",
		Options: []domain.OptionInput{{Text: "A", IsCorrect: true}, {Text: "B"}}}
	for _, in := range []domain.Input{tfng, choice, matchingInput()} {
		in.SampleAnswer = ptr("")
		created := createQuestion(t, svc, author, in)
		if created.SampleAnswer != nil {
			t.Errorf("%s: sample answer = %q, want none", in.Type, *created.SampleAnswer)
		}
		in.Options = optionInputs(created)
		updated := updateQuestion(t, svc, author, created.ID, in)
		if updated.SampleAnswer != nil {
			t.Errorf("%s: sample answer after an update = %q, want none", in.Type, *updated.SampleAnswer)
		}
	}
}
