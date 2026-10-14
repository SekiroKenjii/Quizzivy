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

func TestAVariantLeavesAGroupsMembersOutAndKeepsTheStandaloneQuestionsNumber(t *testing.T) {
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
	if len(got.Changes) != 1 || got.Changes[0].Number != 3 || got.Changes[0].QuestionID != standalone.ID {
		t.Fatalf("changes %+v, want only the standalone question's, still numbered 3: a member is never varied, whatever the assistant proposes", got.Changes)
	}
	sent := o.completer.text()
	for _, secret := range []string{"When did the harbour open?", "What did the harbour handle?", harbourPassage} {
		if strings.Contains(sent, secret) {
			t.Errorf("the variant request carries %q of a group's member", secret)
		}
	}
	if !strings.Contains(sent, "What did planners see parks as?") {
		t.Error("the variant request lacks the standalone question")
	}
}

func TestAVariantOfATestWhoseQuestionsAreAllGroupMembersHasNothingToRead(t *testing.T) {
	o := newOps(t, true)
	membersOnly(o)

	_, err := o.app.Queries.Variant.Handle(context.Background(), query.Variant{Actor: teacher, TestID: "t-1", Changes: []string{"reword"}, Difficulty: "same"})
	if !errors.Is(err, domain.ErrNothingToRead) {
		t.Fatalf("err = %v, want ErrNothingToRead", err)
	}
	if len(o.completer.requests) != 0 || len(o.repo.reserved) != 0 {
		t.Errorf("asked the assistant %d times and reserved %d times for a variant with nothing to vary", len(o.completer.requests), len(o.repo.reserved))
	}
}
