package application_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"quizzivy/internal/modules/ai/application/ports"
	"quizzivy/internal/modules/ai/application/query"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/modules/ai/domain/prompts"
)

func TestGenerateWritesTfngFromAPassageAndRefusesItStandalone(t *testing.T) {
	o := newOps(t, true)
	o.completer.object = func(ports.Request) string {
		return `{"questions":[{"type":"tfng","prompt":"Planners once saw parks as roads.","options":["True","False","Not given"],"answers":["False"],"explanation":""}]}`
	}
	got, err := o.app.Queries.Generate.Handle(context.Background(), query.Generate{Actor: teacher, Source: sourceText, Types: []string{prompts.Tfng}, Count: 1, Level: "b1", Language: domain.English})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 1 || got.Items[0].Type != prompts.Tfng || !slices.Equal(got.Items[0].Options, prompts.TfngOptions()) || !slices.Equal(got.Items[0].AnswerKey, []string{"False"}) {
		t.Errorf("generated %+v, want one tfng question keyed False", got.Items)
	}
	if !strings.Contains(o.completer.text(), "tfng = a statement about the source") || len(o.repo.settled) != 1 {
		t.Errorf("sent %q, settled %d; want the tfng rule sent and the call settled once", o.completer.text(), len(o.repo.settled))
	}

	o = newOps(t, true)
	_, err = o.app.Queries.Generate.Handle(context.Background(), query.Generate{Actor: teacher, Standalone: true, Source: "parks", Types: []string{prompts.SingleChoice, prompts.Tfng}, Count: 3, Level: "b1", Language: domain.English})
	if got := refusedFields(t, err); !slices.Equal(got, []string{"types"}) {
		t.Errorf("a standalone tfng run refused %v, want types", got)
	}
	if len(o.repo.reserved) != 0 || len(o.completer.requests) != 0 {
		t.Errorf("reserved %d, called %d; want the refusal before any credit is held", len(o.repo.reserved), len(o.completer.requests))
	}
}

func TestCheckOffersChangeToNotGivenAndAddWordLimitAsWrites(t *testing.T) {
	o := newOps(t, true)
	o.reads.paper = domain.Paper{Questions: []domain.PaperQuestion{
		{ID: "0198f5f0-0000-7000-8000-0000000000d1", Number: 1, Type: prompts.Tfng, Prompt: "Car-free zones reduce air pollution.", Passage: sourceText, Options: []domain.Option{
			{ID: "0198f5f0-0000-7000-8000-0000000000e1", Text: "True"}, {ID: "0198f5f0-0000-7000-8000-0000000000e2", Text: "False", Correct: true}, {ID: "0198f5f0-0000-7000-8000-0000000000e3", Text: "Not given"},
		}},
		{ID: "0198f5f0-0000-7000-8000-0000000000d2", Number: 2, Type: prompts.ShortAnswer, Prompt: "Why were early maps inaccurate?", Sample: "poor tools"},
	}}
	o.completer.object = func(ports.Request) string {
		return `{"findings":[` +
			`{"question":1,"severity":"likely_error","issue":"The passage never says.","fix":{"kind":"tfng_answer","description":"Change the answer to Not given.","label":"Change to Not given","option":"C","blank":0,"words":0,"text":""}},` +
			`{"question":2,"severity":"minor","issue":"There is no word limit.","fix":{"kind":"word_limit","description":"Add a word limit of three.","label":"Add word limit","option":"","blank":0,"words":3,"text":""}}]}`
	}
	got, err := o.app.Queries.Check.Handle(context.Background(), query.Check{Actor: teacher, TestID: "t", Checks: []string{prompts.CheckKey, prompts.CheckClarity}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Findings) != 2 {
		t.Fatalf("%d findings, want 2", len(got.Findings))
	}
	if f := got.Findings[0].Fix; f.Kind != prompts.FixTfngAnswer || f.OptionID != "0198f5f0-0000-7000-8000-0000000000e3" || f.Text != "Not given" {
		t.Errorf("the tfng fix = %+v, want the Not given option as the new key", f)
	}
	if f := got.Findings[1].Fix; f.Kind != prompts.FixWordLimit || f.MaxWords != 3 || f.Text != "Write NO MORE THAN THREE WORDS." {
		t.Errorf("the word-limit fix = %+v, want a limit of three", f)
	}
	sent := o.completer.text()
	for _, want := range []string{"Q1 [tfng]", "B. False (correct)", "Q2 [short_answer]", "Word limit: none"} {
		if !strings.Contains(sent, want) {
			t.Errorf("the prompt lacks %q:\n%s", want, sent)
		}
	}
	if strings.Contains(sent, "0198f5f0-0000-7000-8000-0000000000e3") || strings.Contains(sent, "0198f5f0-0000-7000-8000-0000000000d1") {
		t.Error("an option or question id reached the prompt")
	}
}

func TestSuggestAScoreGivesAnEssayABandWithItsCriteriaAsReasons(t *testing.T) {
	o := newOps(t, true)
	o.reads.answer = domain.GradedAnswer{
		Type: prompts.Essay, Prompt: "Summarise the chart.", Points: 9, Answer: "The chart shows a considerable rise.", Manual: true,
		Criteria: []domain.Criterion{{ID: "0198f5f0-0000-7000-8000-0000000000f1", Label: "Task achievement"}, {ID: "0198f5f0-0000-7000-8000-0000000000f2", Label: "Grammar"}},
	}
	o.completer.object = func(ports.Request) string {
		return `{"band":7,"criteria":[{"criterion":1,"band":6,"reason":"Few figures."},{"criterion":2,"band":7,"reason":"Mostly accurate."}],"confidence":"medium","reasons":[],"comment":"Add the key figures."}`
	}
	got, err := o.app.Queries.Score.Handle(context.Background(), query.Score{Actor: teacher, Grants: graders, AttemptID: "a", QuestionID: "q"})
	if err != nil {
		t.Fatal(err)
	}
	s := got.Suggestion
	if s.Band == nil || *s.Band != 6.5 || s.Score != 6.5 || !slices.Equal(s.Reasons, []string{"Task achievement: Few figures.", "Grammar: Mostly accurate."}) || s.Comment != "Add the key figures." {
		t.Errorf("suggestion %+v (band %v), want band 6.5 from the criteria with one reason each", s, s.Band)
	}
	if !slices.Equal(s.Criteria, []prompts.CriterionBand{{CriterionID: "0198f5f0-0000-7000-8000-0000000000f1", Band: 6}, {CriterionID: "0198f5f0-0000-7000-8000-0000000000f2", Band: 7}}) {
		t.Errorf("criteria %+v", s.Criteria)
	}
	sent := o.completer.text()
	if !strings.Contains(sent, "1. Task achievement\n2. Grammar") || strings.Contains(sent, "0198f5f0-0000-7000-8000-0000000000f1") {
		t.Errorf("sent %q; want the criterion labels and never their ids", sent)
	}
	if len(o.repo.reserved) != 1 || o.repo.reserved[0].Feature != domain.FeatureScore || len(o.repo.settled) != 1 {
		t.Errorf("reserved %+v, settled %d; want one score reservation, settled", o.repo.reserved, len(o.repo.settled))
	}

	o = newOps(t, true)
	o.reads.answer = domain.GradedAnswer{Type: prompts.Essay, Prompt: "Write.", Points: 9, Answer: "An essay."}
	if _, err := o.app.Queries.Score.Handle(context.Background(), query.Score{Actor: teacher, Grants: graders, AttemptID: "a", QuestionID: "q"}); !errors.Is(err, domain.ErrNotManual) || len(o.repo.reserved) != 0 {
		t.Errorf("an essay already marked: %v, reserved %d; want ErrNotManual before any credit is held", err, len(o.repo.reserved))
	}
}
