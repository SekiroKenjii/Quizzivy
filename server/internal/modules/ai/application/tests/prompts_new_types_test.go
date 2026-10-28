package application_test

import (
	"errors"
	"fmt"
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
		{"matching", matchingQuestion(), []string{"A → ii (Lan works at a hospital. → nurse); B → i (Minh works at a school. → teacher)", "i. teacher", "item by item"}},
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
		"unkeyed matching":       unkeyed,
		"matching with no items": {Type: prompts.Matching, Prompt: "Match.", Options: unkeyed.Options},
	} {
		if _, err := prompts.Explanation(prompts.ExplanationInput{Question: q, Language: domain.English}); !errors.Is(err, domain.ErrNoKey) {
			t.Errorf("%s: err = %v, want ErrNoKey", name, err)
		}
	}
	essay := domain.Question{Type: prompts.Essay, Prompt: "Write about your city.", Sample: "  "}
	if _, err := prompts.Explanation(prompts.ExplanationInput{Question: essay, Language: domain.English}); !errors.Is(err, domain.ErrNoSample) {
		t.Errorf("an essay with no sample answer: err = %v, want ErrNoSample", err)
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
		"i. teacher", "ii. nurse", "Item A. Lan works at a hospital. → ii", "Item B. Minh works at a school. → (no answer)",
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

func TestGenerateKeepsAMatchingDraftOnlyAsWrittenAndAnEssayWithItsBrief(t *testing.T) {
	in := prompts.GenerateInput{Source: "jobs", Types: []string{prompts.Matching, prompts.Essay}, Count: 8, Level: "b1", Language: domain.English, Standalone: true}
	none := []any{}
	matching := func(prompt string, options []string, items ...[2]string) map[string]any {
		pairs := []map[string]any{}
		for _, item := range items {
			pairs = append(pairs, map[string]any{"text": item[0], "answer": item[1]})
		}
		return map[string]any{"type": "matching", "prompt": prompt, "options": options, "answers": []string{}, "items": pairs, "explanation": ""}
	}
	choices := []string{"a place to learn", "a person who cures", "a vehicle"}
	answer := map[string]any{"questions": []map[string]any{
		matching("Kept: answers match a choice without case.", choices, [2]string{"school", "A place to learn"}, [2]string{"doctor", "a person who cures"}),
		matching("Two choices the same.", []string{"a place", "A  place", "a car"}, [2]string{"school", "a car"}, [2]string{"garage", "a car"}),
		matching("An answer that is no choice.", choices, [2]string{"school", "a place to learn"}, [2]string{"bus", "a train"}),
		matching("Two items the same.", choices, [2]string{"school", "a place to learn"}, [2]string{"School", "a vehicle"}),
		matching("One item only.", choices, [2]string{"school", "a place to learn"}),
		matching("An empty choice.", []string{"a place", " "}, [2]string{"school", "a place"}, [2]string{"college", "a place"}),
		{"type": "essay", "prompt": "Write 150-200 words about a job you would like.", "options": []string{}, "answers": []string{"Names the job, gives two reasons and an example."}, "items": none, "explanation": ""},
		{"type": "essay", "prompt": "An essay with no brief.", "options": []string{}, "answers": []string{}, "items": none, "explanation": ""},
	}}
	p := prompts.Generate(in)
	conforms(t, p.Schema, answer)
	for _, want := range []string{"matching = the prompt says what to match", "essay = the prompt is a writing task", "For matching, items holds each thing to match"} {
		if !strings.Contains(p.User, want) {
			t.Errorf("the prompt lacks %q:\n%s", want, p.User)
		}
	}
	got, err := prompts.DecodeGenerate(marshal(t, answer), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("kept %+v, want the first matching draft and the essay with a brief", got)
	}
	match, essay := got[0], got[1]
	wantPairs := []prompts.MatchPair{{Text: "school", Answer: "a place to learn"}, {Text: "doctor", Answer: "a person who cures"}}
	if !slices.Equal(match.Options, choices) || !slices.Equal(match.MatchItems, wantPairs) ||
		!slices.Equal(match.AnswerKey, []string{"a place to learn", "a person who cures"}) {
		t.Errorf("matching draft %+v: its choices, each item keyed to the choice it names, the key the choices used", match)
	}
	if essay.Type != prompts.Essay || !slices.Equal(essay.AnswerKey, []string{"Names the job, gives two reasons and an example."}) || len(essay.Options) != 0 {
		t.Errorf("essay draft %+v", essay)
	}
}

func TestAMatchingDraftOverEightItemsOrTwelveChoicesIsDroppedAndOtherTypesKeepEightOptions(t *testing.T) {
	in := prompts.GenerateInput{Source: sourceText, Types: []string{prompts.Matching, prompts.SingleChoice}, Count: 8, Level: "b1", Language: domain.English}
	draft := func(options, items int) map[string]any {
		var texts []string
		var pairs []map[string]any
		for i := range options {
			texts = append(texts, strings.Repeat("c", i+1))
		}
		for i := range items {
			pairs = append(pairs, map[string]any{"text": strings.Repeat("i", i+1), "answer": texts[i%options]})
		}
		return map[string]any{"type": "matching", "prompt": fmt.Sprintf("%d choices, %d items", options, items), "options": texts, "answers": []string{}, "items": pairs, "explanation": ""}
	}
	var many []string
	for i := range 10 {
		many = append(many, strings.Repeat("o", i+1))
	}
	answer := map[string]any{"questions": []map[string]any{
		draft(12, 8), draft(13, 8), draft(12, 9),
		{"type": "single_choice", "prompt": "Ten options.", "options": many, "answers": []string{"o"}, "items": []any{}, "explanation": ""},
	}}
	got, err := prompts.DecodeGenerate(marshal(t, answer), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Prompt != "12 choices, 8 items" || len(got[0].Options) != 12 || len(got[0].MatchItems) != 8 {
		t.Fatalf("kept %+v, want only the matching draft within 12 choices and 8 items, then the single choice", got)
	}
	if len(got[1].Options) != 8 {
		t.Errorf("a single choice kept %d options, want at most 8", len(got[1].Options))
	}
}
