package domain_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"quizzivy/internal/modules/questions/domain"
)

// T-R6.3: each new type accepts its own fields and refuses every field that
// belongs to another type, with the field error naming the rule that fired.

func intp(n int) *int    { return &n }
func boolp(b bool) *bool { return &b }
func opts(correct ...bool) []domain.OptionInput {
	out := make([]domain.OptionInput, len(correct))
	for i, c := range correct {
		out[i] = domain.OptionInput{Text: string(rune('A' + i)), IsCorrect: c}
	}
	return out
}

func tfng() domain.Input {
	return domain.Input{Type: domain.TFNG, Prompt: "The writer agrees.", Points: "1", Options: opts(false, true, false)}
}

func chooseTwo() domain.Input {
	return domain.Input{Type: domain.MultipleChoice, Prompt: "Choose two.", Points: "2", Options: opts(true, false, true, false), SelectCount: intp(2)}
}

func shortWithLimit() domain.Input {
	return domain.Input{Type: domain.ShortAnswer, Prompt: "No more than two words.", Points: "1", MaxWords: intp(2)}
}

func essayWithRubric() domain.Input {
	return domain.Input{Type: domain.Essay, Prompt: "Write about a city.", Points: "9", SampleAnswer: text("A sample"),
		Rubric: []domain.RubricCriterionInput{{Label: "Task response"}, {Label: "Coherence"}}}
}

func matchingHeadings() domain.Input {
	return domain.Input{Type: domain.Matching, Prompt: "Match the headings.", Points: "3", MatchAllowReuse: boolp(false),
		Options:    opts(false, false, false),
		MatchItems: []domain.MatchItemInput{{Text: "Paragraph A", CorrectOption: 2}, {Text: "Paragraph B", CorrectOption: 0}}}
}

func TestEachNewTypeAcceptsItsOwnFields(t *testing.T) {
	for name, in := range map[string]domain.Input{
		"tfng": tfng(), "choose two": chooseTwo(), "short answer with a word limit": shortWithLimit(),
		"essay with a rubric and a sample": essayWithRubric(), "matching": matchingHeadings(),
		"essay with no rubric": {Type: domain.Essay, Prompt: "Write.", Points: "9"},
	} {
		if err := in.Validate(nil); err != nil {
			t.Errorf("%s was refused: %v", name, fields(t, err))
		}
	}
}

func TestEachTypeRefusesTheRules(t *testing.T) {
	for _, c := range []struct {
		name  string
		input func() domain.Input
		edit  func(*domain.Input)
		field string
	}{
		{"tfng with two options", tfng, func(in *domain.Input) { in.Options = in.Options[:2] }, "options"},
		{"tfng with four options", tfng, func(in *domain.Input) { in.Options = append(in.Options, domain.OptionInput{Text: "D"}) }, "options"},
		{"tfng with no correct option", tfng, func(in *domain.Input) { in.Options[1].IsCorrect = false }, "options"},
		{"tfng with two correct options", tfng, func(in *domain.Input) { in.Options[0].IsCorrect = true }, "options"},
		{"choose N of 1", chooseTwo, func(in *domain.Input) { in.SelectCount = intp(1) }, "selectCount"},
		{"choose N of every option", chooseTwo, func(in *domain.Input) { in.SelectCount = intp(4) }, "selectCount"},
		{"choose N above seven", chooseTwo, func(in *domain.Input) { in.Options = opts(make([]bool, 9)...); in.SelectCount = intp(8) }, "selectCount"},
		{"choose two with three correct", chooseTwo, func(in *domain.Input) { in.Options[1].IsCorrect = true }, "options"},
		{"choose two with one correct", chooseTwo, func(in *domain.Input) { in.Options[0].IsCorrect = false }, "options"},
		{"selectCount on a single choice", chooseTwo, func(in *domain.Input) { in.Type = domain.SingleChoice }, "selectCount"},
		{"word limit of 0", shortWithLimit, func(in *domain.Input) { in.MaxWords = intp(0) }, "maxWords"},
		{"word limit of 101", shortWithLimit, func(in *domain.Input) { in.MaxWords = intp(101) }, "maxWords"},
		{"word limit on an essay", essayWithRubric, func(in *domain.Input) { in.MaxWords = intp(250) }, "maxWords"},
		{"essay with options", essayWithRubric, func(in *domain.Input) { in.Options = opts(true, false) }, "options"},
		{"essay with blanks", essayWithRubric, func(in *domain.Input) {
			in.Blanks = []domain.BlankInput{{Ordinal: 1, AcceptedAnswers: []string{"a"}}}
		}, "blanks"},
		{"essay with seven criteria", essayWithRubric, func(in *domain.Input) {
			in.Rubric = nil
			for _, label := range []string{"a", "b", "c", "d", "e", "f", "g"} {
				in.Rubric = append(in.Rubric, domain.RubricCriterionInput{Label: label})
			}
		}, "rubric"},
		{"a blank criterion", essayWithRubric, func(in *domain.Input) { in.Rubric[1].Label = "  " }, "rubric[1].label"},
		{"a criterion of 81 characters", essayWithRubric, func(in *domain.Input) { in.Rubric[1].Label = strings.Repeat("x", 81) }, "rubric[1].label"},
		{"two criteria differing in case", essayWithRubric, func(in *domain.Input) { in.Rubric[1].Label = " task RESPONSE" }, "rubric[1].label"},
		{"a rubric on a short answer", shortWithLimit, func(in *domain.Input) {
			in.Rubric = []domain.RubricCriterionInput{{Label: "x"}}
		}, "rubric"},
		{"matching with one choice", matchingHeadings, func(in *domain.Input) {
			in.Options = in.Options[:1]
			in.MatchItems[0].CorrectOption = 0
		}, "options"},
		{"matching with thirteen choices", matchingHeadings, func(in *domain.Input) { in.Options = opts(make([]bool, 13)...) }, "options"},
		{"a matching choice marked correct", matchingHeadings, func(in *domain.Input) { in.Options[0].IsCorrect = true }, "options"},
		{"matching with one item", matchingHeadings, func(in *domain.Input) { in.MatchItems = in.MatchItems[:1] }, "matchItems"},
		{"matching with 21 items", matchingHeadings, func(in *domain.Input) {
			in.MatchAllowReuse = boolp(true)
			for len(in.MatchItems) < 21 {
				in.MatchItems = append(in.MatchItems, domain.MatchItemInput{Text: "x", CorrectOption: 0})
			}
		}, "matchItems"},
		{"a blank item", matchingHeadings, func(in *domain.Input) { in.MatchItems[1].Text = " " }, "matchItems[1].text"},
		{"an item of 501 characters", matchingHeadings, func(in *domain.Input) { in.MatchItems[1].Text = strings.Repeat("x", 501) }, "matchItems[1].text"},
		{"an item keyed past the choices", matchingHeadings, func(in *domain.Input) { in.MatchItems[1].CorrectOption = 3 }, "matchItems[1].correctOption"},
		{"an item keyed below zero", matchingHeadings, func(in *domain.Input) { in.MatchItems[1].CorrectOption = -1 }, "matchItems[1].correctOption"},
		{"two items on one choice without reuse", matchingHeadings, func(in *domain.Input) { in.MatchItems[1].CorrectOption = 2 }, "matchItems[1].correctOption"},
		{"more items than choices without reuse", matchingHeadings, func(in *domain.Input) {
			in.MatchItems = append(in.MatchItems, domain.MatchItemInput{Text: "C", CorrectOption: 1}, domain.MatchItemInput{Text: "D", CorrectOption: 1})
		}, "matchItems"},
		{"matching without matchAllowReuse", matchingHeadings, func(in *domain.Input) { in.MatchAllowReuse = nil }, "matchAllowReuse"},
		{"matchAllowReuse on a single choice", tfng, func(in *domain.Input) { in.MatchAllowReuse = boolp(false) }, "matchAllowReuse"},
		{"items on a tfng", tfng, func(in *domain.Input) { in.MatchItems = []domain.MatchItemInput{{Text: "x"}} }, "matchItems"},
		{"a sample answer on matching", matchingHeadings, func(in *domain.Input) { in.SampleAnswer = text("x") }, "sampleAnswer"},
		{"an image inside a prompt", essayWithRubric, func(in *domain.Input) {
			in.PromptContent = json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"image","assetId":"01a04900-0000-7000-8000-000000000001","alt":"A map"}]}`)
		}, "promptContent"},
		{"audio inside an explanation", essayWithRubric, func(in *domain.Input) {
			in.Explanation = text("")
			in.ExplanationContent = json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"audio","assetId":"01a04900-0000-7000-8000-000000000001","label":"Track 1"}]}`)
		}, "explanationContent"},
	} {
		in := c.input()
		c.edit(&in)
		err := in.Validate(nil)
		if !hasField(t, err, c.field) {
			t.Errorf("%s: want %s refused, got %v", c.name, c.field, fields(t, err))
		}
	}
}

func TestReuseLetsTwoItemsShareAChoice(t *testing.T) {
	in := matchingHeadings()
	in.MatchAllowReuse = boolp(true)
	in.Options = in.Options[:2]
	in.MatchItems = []domain.MatchItemInput{{Text: "A", CorrectOption: 0}, {Text: "B", CorrectOption: 0}, {Text: "C", CorrectOption: 1}}
	if err := in.Validate(nil); err != nil {
		t.Errorf("reuse with more items than choices was refused: %v", fields(t, err))
	}
}

func TestEveryNewRefusalSpeaksEnglishToo(t *testing.T) {
	in := matchingHeadings()
	in.MatchAllowReuse = nil
	in.MatchItems[1].CorrectOption = 9
	in.Rubric = []domain.RubricCriterionInput{{Label: "x"}}
	in.SelectCount, in.MaxWords = intp(2), intp(2)
	err := in.Validate(nil)
	var invalid *domain.ValidationError
	if !errors.As(err, &invalid) || len(invalid.Fields) < 5 {
		t.Fatalf("want five refusals, got %v", err)
	}
	for _, f := range invalid.Fields {
		if f.English == "" || f.Message == "" {
			t.Errorf("%s is refused in one language only: %+v", f.Field, f)
		}
	}
}

func TestMediaOutsideTheRulesIsRefusedOnItsField(t *testing.T) {
	prompt := essayWithRubric()
	prompt.PromptContent = json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"audio","assetId":"01a04900-0000-7000-8000-000000000001"}]}`)
	explanation := essayWithRubric()
	explanation.Explanation = text("")
	explanation.ExplanationContent = json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"audio","assetId":"01a04900-0000-7000-8000-000000000001","label":"Track 1"}]}`)
	for name, tc := range map[string]struct {
		in    domain.Input
		field string
	}{"audio naming an asset in the prompt": {prompt, "promptContent"}, "a labelled audio in the explanation": {explanation, "explanationContent"}} {
		var invalid *domain.ValidationError
		if !errors.As(tc.in.Validate(nil), &invalid) {
			t.Fatalf("%s was accepted", name)
		}
		if f := invalid.Fields[0]; f.Field != tc.field || f.Message == "" || f.English == "" {
			t.Errorf("%s: refused as %+v, want %s in vi and en", name, f, tc.field)
		}
	}
}
