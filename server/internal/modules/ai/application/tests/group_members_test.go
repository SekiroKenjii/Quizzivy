package application_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"quizzivy/internal/modules/ai/application/ports"
	"quizzivy/internal/modules/ai/application/query"
	"quizzivy/internal/modules/ai/domain"
)

const (
	harbourPassage = "The harbour opened in 1902 and handled mostly salt and timber."
	harbourGroupID = "g-harbour"
)

func harbourMember(number int, prompt string) domain.PaperQuestion {
	return domain.PaperQuestion{
		ID: fmt.Sprintf("m-%d", number), Number: number, GroupID: harbourGroupID, Type: "single_choice", Prompt: prompt, Passage: harbourPassage, OrderFixed: true,
		Options: []domain.Option{{ID: fmt.Sprintf("m-%d-a", number), Text: "In 1902", Correct: true}, {ID: fmt.Sprintf("m-%d-b", number), Text: "In 1920"}},
	}
}

func membersOnly(o ops) {
	o.reads.paper = domain.Paper{Questions: []domain.PaperQuestion{
		harbourMember(1, "When did the harbour open?"),
		harbourMember(2, "What did the harbour handle?"),
	}}
}

func TestACheckOfATestWhoseQuestionsAreAllGroupMembersReadsThemInsteadOfFindingNothing(t *testing.T) {
	o := newOps(t, true)
	membersOnly(o)

	got, err := o.app.Queries.Check.Handle(context.Background(), query.Check{Actor: teacher, TestID: "t-1", Checks: []string{"key"}})
	if errors.Is(err, domain.ErrNothingToRead) {
		t.Fatal("a test made only of group members answered that there is nothing to read")
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Findings) != 1 || got.Findings[0].Number != 1 {
		t.Errorf("findings %+v, want the one the assistant made about question 1", got.Findings)
	}
	sent := o.completer.text()
	for _, want := range []string{"When did the harbour open?", "What did the harbour handle?", "Q1 [single_choice] (uses P1)", "Q2 [single_choice] (uses P1)"} {
		if !strings.Contains(sent, want) {
			t.Errorf("the request to the assistant lacks %q", want)
		}
	}
	if n := strings.Count(sent, harbourPassage); n != 1 {
		t.Errorf("the passage is sent %d times, want once for the group's two members", n)
	}
}

func TestACheckReadsAStandaloneQuestionAndAGroupsMembersTogetherUnderTheirBuilderNumbers(t *testing.T) {
	o := newOps(t, true)
	standalone := o.reads.paper.Questions[0]
	o.reads.paper = domain.Paper{Questions: []domain.PaperQuestion{standalone, harbourMember(2, "When did the harbour open?"), harbourMember(3, "What did the harbour handle?")}}

	if _, err := o.app.Queries.Check.Handle(context.Background(), query.Check{Actor: teacher, TestID: "t-1", Checks: []string{"key"}}); err != nil {
		t.Fatal(err)
	}
	sent := o.completer.text()
	for _, want := range []string{"What did planners see parks as?", "Q2 [single_choice] (uses P1)", "Q3 [single_choice] (uses P1)", "When did the harbour open?"} {
		if !strings.Contains(sent, want) {
			t.Errorf("the request to the assistant lacks %q", want)
		}
	}
}

func TestAVariantVariesAGroupsMembersAndAStandaloneQuestionUnderTheirBuilderNumbers(t *testing.T) {
	o := newOps(t, true)
	standalone := o.reads.paper.Questions[0]
	standalone.Number = 3
	o.reads.paper = domain.Paper{Questions: []domain.PaperQuestion{harbourMember(1, "When did the harbour open?"), harbourMember(2, "What did the harbour handle?"), standalone}}
	o.completer.object = func(ports.Request) string {
		return `{"changes":[
			{"question":1,"kind":"reword","prompt":"In what year did the harbour open?","order":[],"option":"","text":""},
			{"question":3,"kind":"reword","prompt":"What were parks thought to be?","order":[],"option":"","text":""}]}`
	}

	got, err := o.app.Queries.Variant.Handle(context.Background(), query.Variant{Actor: teacher, TestID: "t-1", Changes: []string{"reword"}, Difficulty: "same"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Changes) != 2 || got.Changes[0].Number != 1 || got.Changes[0].QuestionID != "m-1" || got.Changes[1].Number != 3 || got.Changes[1].QuestionID != standalone.ID {
		t.Fatalf("changes %+v, want the member's as question 1 and the standalone question's as question 3", got.Changes)
	}
	sent := o.completer.text()
	for _, want := range []string{"When did the harbour open?", "Q1 [single_choice] (uses P1)", "What did planners see parks as?"} {
		if !strings.Contains(sent, want) {
			t.Errorf("the variant request lacks %q", want)
		}
	}
	if n := strings.Count(sent, harbourPassage); n != 1 {
		t.Errorf("the passage is sent %d times, want once for the group's two members", n)
	}
}

func TestAVariantNeitherReordersAFixedMemberNorRewordsAGappedOne(t *testing.T) {
	o := newOps(t, true)
	free := harbourMember(1, "When did the harbour open?")
	free.OrderFixed = false
	free.Options = append(free.Options, domain.Option{ID: "m-1-c", Text: "In 1950"})
	fixed := harbourMember(2, "What did the harbour handle?")
	fixed.Options = append(fixed.Options, domain.Option{ID: "m-2-c", Text: "Coal"})
	gapped := harbourMember(3, "Gap 1")
	gapped.Gapped = true
	o.reads.paper = domain.Paper{Questions: []domain.PaperQuestion{free, fixed, gapped}}
	o.completer.object = func(ports.Request) string {
		return `{"changes":[
			{"question":1,"kind":"reorder","prompt":"","order":["C","A","B"],"option":"","text":""},
			{"question":2,"kind":"reorder","prompt":"","order":["C","A","B"],"option":"","text":""},
			{"question":3,"kind":"reword","prompt":"The first gap","order":[],"option":"","text":""}]}`
	}

	got, err := o.app.Queries.Variant.Handle(context.Background(), query.Variant{Actor: teacher, TestID: "t-1", Changes: []string{"reword", "reorder"}, Difficulty: "same"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Changes) != 1 || got.Changes[0].Number != 1 || got.Changes[0].Kind != "reorder" {
		t.Fatalf("changes %+v, want only the reorder of the member whose order is free", got.Changes)
	}
}

func TestAVariantOfATestWhoseQuestionsAreAllGroupMembersVariesThem(t *testing.T) {
	o := newOps(t, true)
	membersOnly(o)
	o.completer.object = func(ports.Request) string {
		return `{"changes":[{"question":2,"kind":"reword","prompt":"What goods passed through the harbour?","order":[],"option":"","text":""}]}`
	}

	got, err := o.app.Queries.Variant.Handle(context.Background(), query.Variant{Actor: teacher, TestID: "t-1", Changes: []string{"reword"}, Difficulty: "same"})
	if err != nil {
		t.Fatalf("a test made only of group members: %v", err)
	}
	if len(got.Changes) != 1 || got.Changes[0].QuestionID != "m-2" {
		t.Errorf("changes %+v, want the second member's", got.Changes)
	}
}
