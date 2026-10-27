package application_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/modules/ai/domain/prompts"
)

func matchingQuestion() domain.Question {
	return domain.Question{
		ID: "q-match", Type: prompts.Matching, Prompt: "Match each person with the job they do.",
		Options:    []domain.Option{{ID: "o1", Text: "teacher"}, {ID: "o2", Text: "nurse"}, {ID: "o3", Text: "driver"}},
		MatchItems: []domain.MatchItem{{Text: "Lan works at a hospital.", Option: 1}, {Text: "Minh works at a school.", Option: 0}},
	}
}

func TestAnExplanationReadsTheKeyOfATfngAMatchingAndAnEssayQuestion(t *testing.T) {
	tfng := domain.Question{Type: prompts.Tfng, Prompt: "The park opened in 1990.", Options: []domain.Option{{Text: "True"}, {Text: "False"}, {Text: "Not given", Correct: true}}}
	essay := domain.Question{Type: prompts.Essay, Prompt: "Write about your city.", Sample: "Names the city, two places and one reason to visit."}
	cases := []struct {
		name string
		q    domain.Question
		want []string
	}{
		{"tfng", tfng, []string{"Correct answer: Not given"}},
		{"matching", matchingQuestion(), []string{"1. Lan works at a hospital. -> B. nurse; 2. Minh works at a school. -> A. teacher", "item by item"}},
		{"essay", essay, []string{"What a strong answer covers: Names the city", "how it is organised"}},
	}
	for _, c := range cases {
		p, err := prompts.Explanation(prompts.ExplanationInput{Question: c.q, Language: domain.English})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		for _, want := range c.want {
			if !strings.Contains(p.User, want) {
				t.Errorf("%s: the prompt lacks %q:\n%s", c.name, want, p.User)
			}
		}
		if strings.Contains(p.User, "most tempting wrong option") && c.name != "tfng" {
			t.Errorf("%s: the prompt asks about a wrong option:\n%s", c.name, p.User)
		}
	}
}

func TestAnExplanationRefusesAMatchingQuestionWithAnItemLeftUnkeyedAndAnEssayWithNoSample(t *testing.T) {
	unkeyed := matchingQuestion()
	unkeyed.MatchItems[1].Option = -1
	for name, q := range map[string]domain.Question{
		"unkeyed matching": unkeyed,
		"matching with no items": {Type: prompts.Matching, Prompt: "Match.", Options: unkeyed.Options},
		"essay with no sample":   {Type: prompts.Essay, Prompt: "Write about your city."},
	} {
		if _, err := prompts.Explanation(prompts.ExplanationInput{Question: q, Language: domain.English}); !errors.Is(err, domain.ErrNoKey) {
			t.Errorf("%s: err = %v, want ErrNoKey", name, err)
		}
	}
}

func TestTheCheckPromptSendsMatchingKeysChooseNAndRubricCriteria(t *testing.T) {
	questions := []domain.PaperQuestion{
		{ID: "a", Number: 1, Type: prompts.Matching, Prompt: "Match each person with the job.",
			Options:    []domain.Option{{ID: "o1", Text: "teacher"}, {ID: "o2", Text: "nurse"}},
			MatchItems: []domain.MatchItem{{Text: "Lan works at a hospital.", Option: 1}, {Text: "Minh works at a school.", Option: -1}}},
		{ID: "b", Number: 2, Type: prompts.MultipleChoice, Prompt: "Choose TWO.", SelectCount: 2,
			Options: []domain.Option{{ID: "x", Text: "red", Correct: true}, {ID: "y", Text: "blue", Correct: true}, {ID: "z", Text: "green"}}},
		{ID: "c", Number: 3, Type: prompts.Essay, Prompt: "Write about your city.", Sample: "Two places.",
			Criteria: []domain.Criterion{{ID: "k1", Label: "Task response"}, {ID: "k2", Label: "Coherence"}}},
	}
	p, err := prompts.Check(prompts.CheckInput{Questions: questions, Checks: []string{prompts.CheckKey}, Language: domain.English})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Item 1. Lan works at a hospital. -> B", "Item 2. Minh works at a school. -> (no answer)",
		"Choose exactly 2 options", "Rubric criteria: Task response | Coherence",
		"a matching item keyed to an option that does not fit it",
	} {
		if !strings.Contains(p.User, want) {
			t.Errorf("the prompt lacks %q:\n%s", want, p.User)
		}
	}
}

func TestAnInsightNamesTheNewTypesForAPracticeSet(t *testing.T) {
	in := prompts.InsightsInput{Language: domain.English, Results: domain.Results{HandedIn: 3, Questions: []domain.ResultQuestion{
		{Number: 1, Type: prompts.Tfng, Prompt: "The park opened in 1990.", Key: "Not given"},
		{Number: 2, Type: prompts.Matching, Prompt: "Match the jobs.", Key: "1: B; 2: A"},
		{Number: 3, Type: prompts.Essay, Prompt: "Write about your city."},
		{Number: 4, Type: prompts.SingleChoice, Prompt: "Pick one.", Key: "A"},
	}}}
	answer := map[string]any{"insights": []map[string]any{
		{"kind": "problem", "title": "Not given read as False", "body": "Two of three papers.", "action": "practice", "questions": []int{1, 2, 3, 4}},
	}}
	p, err := prompts.Insights(in)
	if err != nil {
		t.Fatal(err)
	}
	conforms(t, p.Schema, answer)
	got, err := prompts.DecodeInsights(marshal(t, answer), in)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{prompts.SingleChoice, prompts.Tfng, prompts.Matching, prompts.Essay}
	if len(got) != 1 || !slices.Equal(got[0].Types, want) {
		t.Errorf("the insight's types read %v, want %v", got, want)
	}
}
