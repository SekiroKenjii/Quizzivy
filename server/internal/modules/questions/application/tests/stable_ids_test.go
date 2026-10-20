//go:build integration

package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"quizzivy/internal/modules/questions/application"
	"quizzivy/internal/modules/questions/application/command"
	"quizzivy/internal/modules/questions/application/query"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/shared/validation"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func createQuestion(t *testing.T, svc *application.Application, author string, in domain.Input) domain.Question {
	t.Helper()
	q, err := svc.Commands.Create.Handle(context.Background(), command.Create{Request: domain.WriteRequest{Input: in, ActorID: author}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return q
}

func updateQuestion(t *testing.T, svc *application.Application, author, id string, in domain.Input) domain.Question {
	t.Helper()
	q, err := svc.Commands.Update.Handle(context.Background(), command.Update{Request: domain.WriteRequest{ID: id, Input: in, ActorID: author}})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	return q
}

func reread(t *testing.T, svc *application.Application, id string) domain.Question {
	t.Helper()
	q, err := svc.Queries.Get.Handle(context.Background(), query.Get{Scope: everyone, ID: id})
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func optionIDs(q domain.Question) []string {
	ids := make([]string, len(q.Options))
	for i, o := range q.Options {
		ids[i] = o.ID
	}
	return ids
}

func withOptionIDs(in domain.Input, ids ...string) domain.Input {
	for i := range in.Options {
		if ids[i] != "" {
			in.Options[i].ID = &ids[i]
		}
	}
	return in
}

func rowsWithID(t *testing.T, pool *pgxpool.Pool, table, id string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM `+table+` WHERE id = $1`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAnOptionKeepsItsIdWhenItsQuestionIsSaved(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)

	created := createQuestion(t, svc, author, choiceInput("Thủ đô của Việt Nam là gì?", "Hà Nội", "Huế", "Đà Nẵng"))
	before := optionIDs(created)

	edited := withOptionIDs(choiceInput("Thủ đô của Việt Nam là gì?", "Hà Nội", "Huế (cố đô)", "Đà Nẵng"), before...)
	saved := updateQuestion(t, svc, author, created.ID, edited)

	if got := optionIDs(saved); !slices.Equal(got, before) {
		t.Fatalf("option ids after a save = %v, want the same %v", got, before)
	}
	assertOptionOrder(t, saved, []string{"Hà Nội", "Huế (cố đô)", "Đà Nẵng"}, []bool{true, false, false})
	if got := optionIDs(reread(t, svc, created.ID)); !slices.Equal(got, before) {
		t.Errorf("option ids read back = %v, want %v", got, before)
	}
}

func TestOptionIdsFollowTheirOptionsThroughAReorder(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)

	created := createQuestion(t, svc, author, choiceInput("Chọn", "A", "B", "C", "D"))
	id := optionIDs(created)

	rotated := domain.Input{Type: domain.SingleChoice, Prompt: "Chọn", Points: "2.50", Tags: []string{}, Options: []domain.OptionInput{
		{Text: "D"}, {Text: "A", IsCorrect: true}, {Text: "B"}, {Text: "C"},
	}}
	saved := updateQuestion(t, svc, author, created.ID, withOptionIDs(rotated, id[3], id[0], id[1], id[2]))

	if got, want := optionIDs(saved), []string{id[3], id[0], id[1], id[2]}; !slices.Equal(got, want) {
		t.Fatalf("option ids after a reorder = %v, want %v", got, want)
	}
	assertOptionOrder(t, saved, []string{"D", "A", "B", "C"}, []bool{false, true, false, false})
}

func TestAnOptionSentWithoutAnIdIsCreatedAndOneNotSentIsDeleted(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)

	created := createQuestion(t, svc, author, choiceInput("Chọn", "A", "B", "C"))
	id := optionIDs(created)

	in := withOptionIDs(choiceInput("Chọn", "A", "D"), id[0], "")
	saved := updateQuestion(t, svc, author, created.ID, in)

	assertOptionOrder(t, saved, []string{"A", "D"}, []bool{true, false})
	if saved.Options[0].ID != id[0] {
		t.Errorf("the option sent with its id changed id: %s, want %s", saved.Options[0].ID, id[0])
	}
	if slices.Contains(id, saved.Options[1].ID) {
		t.Errorf("the option sent without an id reused an old id %s", saved.Options[1].ID)
	}
	for _, gone := range id[1:] {
		if n := rowsWithID(t, pool, "app.question_options", gone); n != 0 {
			t.Errorf("option %s was not sent and is still stored", gone)
		}
	}
}

func TestAnIdTheQuestionDoesNotHoldCreatesAndIsNeverTaken(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)

	other := createQuestion(t, svc, author, choiceInput("Câu khác", "X", "Y"))
	otherIDs := optionIDs(other)
	mine := createQuestion(t, svc, author, choiceInput("Câu của tôi", "A", "B"))
	held := optionIDs(mine)
	unknown := uuid.NewString()

	in := withOptionIDs(choiceInput("Câu của tôi", "A", "A lặp lại", "B", "X chép sang", "Mới"),
		strings.ToUpper(held[0]), held[0], held[1], otherIDs[0], unknown)
	saved := updateQuestion(t, svc, author, mine.ID, in)

	got := optionIDs(saved)
	if got[0] != held[0] || got[2] != held[1] {
		t.Errorf("kept ids = %s, %s; want %s (sent in capitals) and %s", got[0], got[2], held[0], held[1])
	}
	for _, i := range []int{1, 3, 4} {
		if slices.Contains(held, got[i]) || slices.Contains(otherIDs, got[i]) || got[i] == unknown {
			t.Errorf("option %d (%q) took the id %s it was sent instead of getting its own", i, saved.Options[i].Text, got[i])
		}
	}
	untouched := reread(t, svc, other.ID)
	assertOptionOrder(t, untouched, []string{"X", "Y"}, []bool{true, false})
	if !slices.Equal(optionIDs(untouched), otherIDs) {
		t.Errorf("the other question's options changed ids: %v, want %v", optionIDs(untouched), otherIDs)
	}
}

func TestACreateIgnoresTheIdsItIsSent(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)

	source := createQuestion(t, svc, author, choiceInput("Câu gốc", "A", "B"))
	sourceIDs := optionIDs(source)

	copied := createQuestion(t, svc, author, withOptionIDs(choiceInput("Bản chép", "A", "B"), sourceIDs...))

	assertOptionOrder(t, copied, []string{"A", "B"}, []bool{true, false})
	for _, id := range optionIDs(copied) {
		if slices.Contains(sourceIDs, id) {
			t.Errorf("the new question holds the source's option id %s", id)
		}
	}
	if !slices.Equal(optionIDs(reread(t, svc, source.ID)), sourceIDs) {
		t.Error("creating from another question's ids changed the source")
	}
}

func TestChangingTheTypeDropsTheOptionsAndBlanksItNoLongerHas(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)

	created := createQuestion(t, svc, author, choiceInput("Chọn", "A", "B"))
	before := optionIDs(created)

	short := domain.Input{Type: domain.ShortAnswer, Prompt: "Chọn", Points: "2.50", Tags: []string{}}
	saved := updateQuestion(t, svc, author, created.ID, short)
	if len(saved.Options) != 0 {
		t.Fatalf("%d options remain on a short-answer question", len(saved.Options))
	}
	for _, id := range before {
		if n := rowsWithID(t, pool, "app.question_options", id); n != 0 {
			t.Errorf("option %s survived the type change", id)
		}
	}

	back := updateQuestion(t, svc, author, created.ID, withOptionIDs(choiceInput("Chọn", "A", "B"), before...))
	for _, id := range optionIDs(back) {
		if slices.Contains(before, id) {
			t.Errorf("a deleted option id %s came back", id)
		}
	}
}

func TestFormattingFollowsAnOptionByIdAndIsStillRefusedWhenItsTextChanges(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)

	underlined := json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"word","marks":["underline"]}]}]}`)
	in := choiceInput("Phát âm", "word", "other")
	in.Options[0].Content = underlined
	created := createQuestion(t, svc, author, in)
	id := optionIDs(created)

	swapped := domain.Input{Type: domain.SingleChoice, Prompt: "Phát âm", Points: "2.50", Tags: []string{}, Options: []domain.OptionInput{
		{Text: "other"}, {Text: "word", IsCorrect: true},
	}}
	saved := updateQuestion(t, svc, author, created.ID, withOptionIDs(swapped, id[1], id[0]))
	if saved.Options[0].Content != nil {
		t.Errorf("the plain option gained formatting: %s", saved.Options[0].Content)
	}
	if !sameJSON(t, saved.Options[1].Content, underlined) {
		t.Fatalf("the formatted option lost its formatting on a reorder: %s", saved.Options[1].Content)
	}
	if got := optionIDs(saved); got[0] != id[1] || got[1] != id[0] {
		t.Errorf("ids after the reorder = %v, want [%s %s]", got, id[1], id[0])
	}

	retyped := domain.Input{Type: domain.SingleChoice, Prompt: "Phát âm", Points: "2.50", Tags: []string{}, Options: []domain.OptionInput{
		{Text: "other"}, {Text: "words", IsCorrect: true},
	}}
	_, err := svc.Commands.Update.Handle(context.Background(), command.Update{Request: domain.WriteRequest{
		ID: created.ID, Input: withOptionIDs(retyped, id[1], id[0]), ActorID: author,
	}})
	var invalid *validation.Error
	if !errors.As(err, &invalid) || invalid.Fields[0].Field != "options[1].content" {
		t.Fatalf("changing a formatted option's text without its document returned %v, want the options[1].content refusal", err)
	}
	if after := reread(t, svc, created.ID); after.Options[1].Text != "word" || !sameJSON(t, after.Options[1].Content, underlined) {
		t.Error("the refused save changed the stored option")
	}
}

func sameJSON(t *testing.T, got, want json.RawMessage) bool {
	t.Helper()
	var a, b any
	if err := json.Unmarshal(got, &a); err != nil {
		return false
	}
	if err := json.Unmarshal(want, &b); err != nil {
		t.Fatal(err)
	}
	return reflect.DeepEqual(a, b)
}

func blankByID(t *testing.T, q domain.Question, id string) domain.Blank {
	t.Helper()
	for _, b := range q.Blanks {
		if b.ID == id {
			return b
		}
	}
	t.Fatalf("blank %s is not on the question: %+v", id, q.Blanks)
	return domain.Blank{}
}

func TestABlankKeepsItsIdWhileItsAnswersAndPlaceAreReplaced(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)

	created := createQuestion(t, svc, author, domain.Input{
		Type: domain.FillBlank, Prompt: "Tôi {{1}} đi học và {{2}} về nhà", Points: "3.00", Tags: []string{},
		Blanks: []domain.BlankInput{
			{Ordinal: 1, AcceptedAnswers: []string{"đi", "di"}},
			{Ordinal: 2, AcceptedAnswers: []string{"về"}, CaseSensitive: true},
		},
	})
	first, second := created.Blanks[0].ID, created.Blanks[1].ID

	saved := updateQuestion(t, svc, author, created.ID, domain.Input{
		Type: domain.FillBlank, Prompt: "Tôi {{1}} về nhà và {{2}} đi học và {{3}}", Points: "3.00", Tags: []string{},
		Blanks: []domain.BlankInput{
			{ID: &second, Ordinal: 1, AcceptedAnswers: []string{"về", "ve"}},
			{ID: &first, Ordinal: 2, AcceptedAnswers: []string{"đi"}, CaseSensitive: true},
			{Ordinal: 3, AcceptedAnswers: []string{"nữa"}},
		},
	})

	if len(saved.Blanks) != 3 {
		t.Fatalf("%d blanks, want 3", len(saved.Blanks))
	}
	moved := blankByID(t, saved, second)
	if moved.Ordinal != 1 || moved.CaseSensitive || !slices.Equal(moved.AcceptedAnswers, []string{"ve", "về"}) {
		t.Errorf("the second blank, kept by id: %+v; want ordinal 1, not case sensitive, answers ve and về", moved)
	}
	earlier := blankByID(t, saved, first)
	if earlier.Ordinal != 2 || !earlier.CaseSensitive || !slices.Equal(earlier.AcceptedAnswers, []string{"đi"}) {
		t.Errorf("the first blank, kept by id: %+v; want ordinal 2, case sensitive, the one answer đi", earlier)
	}
	added := saved.Blanks[2]
	if added.ID == first || added.ID == second || added.Ordinal != 3 {
		t.Errorf("the blank sent without an id: %+v; want a new id at ordinal 3", added)
	}

	dropped := updateQuestion(t, svc, author, created.ID, domain.Input{
		Type: domain.FillBlank, Prompt: "Tôi {{1}} về nhà", Points: "3.00", Tags: []string{},
		Blanks: []domain.BlankInput{{ID: &second, Ordinal: 1, AcceptedAnswers: []string{"về"}}},
	})
	if len(dropped.Blanks) != 1 || dropped.Blanks[0].ID != second {
		t.Fatalf("blanks after dropping two = %+v, want only %s", dropped.Blanks, second)
	}
	for _, gone := range []string{first, added.ID} {
		if n := rowsWithID(t, pool, "app.question_blanks", gone); n != 0 {
			t.Errorf("blank %s was not sent and is still stored", gone)
		}
	}
}

func TestTwoBlanksCanSwapGapsWhileKeepingTheirIds(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)

	a, b := "a", "b"
	prompt := json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"table","rows":[[{"header":false,"rowSpan":1,"colSpan":1,"content":[{"type":"paragraph","content":[{"type":"gap","id":"b","label":"2"}]}]},{"header":false,"rowSpan":1,"colSpan":1,"content":[{"type":"paragraph","content":[{"type":"gap","id":"a","label":"1"}]}]}]]}]}`)
	input := func(first, second *string) domain.Input {
		return domain.Input{
			Type: domain.FillBlank, Prompt: "[2]\t[1]", PromptContent: prompt, Points: "1.00", Tags: []string{},
			Blanks: []domain.BlankInput{
				{GapID: first, Ordinal: 1, AcceptedAnswers: []string{"one"}},
				{GapID: second, Ordinal: 2, AcceptedAnswers: []string{"two"}},
			},
		}
	}
	created := createQuestion(t, svc, author, input(&a, &b))
	one, two := created.Blanks[0].ID, created.Blanks[1].ID

	swapped := input(&b, &a)
	swapped.Blanks[0].ID, swapped.Blanks[1].ID = &one, &two
	saved := updateQuestion(t, svc, author, created.ID, swapped)

	first, second := blankByID(t, saved, one), blankByID(t, saved, two)
	if first.GapID == nil || *first.GapID != "b" || second.GapID == nil || *second.GapID != "a" {
		t.Errorf("gaps after the swap: %v and %v, want b and a", first.GapID, second.GapID)
	}
	if !slices.Equal(first.AcceptedAnswers, []string{"one"}) || !slices.Equal(second.AcceptedAnswers, []string{"two"}) {
		t.Errorf("answers followed the wrong blank: %v and %v", first.AcceptedAnswers, second.AcceptedAnswers)
	}
}
