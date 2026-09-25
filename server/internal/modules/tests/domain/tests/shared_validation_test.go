package domain_test

import (
	"quizzivy/internal/modules/tests/domain"
	"testing"
)

func TestPublicationRejectsInvalidQuestionStructures(t *testing.T) {
	cases := []struct {
		name   string
		change func(*domain.DraftQuestion)
	}{
		{"blank prompt", func(q *domain.DraftQuestion) { q.Prompt = "   " }},
		{"unknown interaction", func(q *domain.DraftQuestion) { q.Type = "matching" }},
		{"nonfinite score", func(q *domain.DraftQuestion) { q.Points = "NaN" }},
		{"rounded score", func(q *domain.DraftQuestion) { q.Points = "1.001" }},
		{"two keys on true false", func(q *domain.DraftQuestion) {
			q.Type = "true_false"
			q.Options = []domain.DraftOption{{Text: "Đúng", IsCorrect: true}, {Text: "Sai", IsCorrect: true}}
		}},
		{"one option", func(q *domain.DraftQuestion) {
			q.Type = "single_choice"
			q.Options = []domain.DraftOption{{Text: "A", IsCorrect: true}}
		}},
		{"blank option", func(q *domain.DraftQuestion) {
			q.Type = "single_choice"
			q.Options = []domain.DraftOption{{Text: "A", IsCorrect: true}, {Text: " "}}
		}},
		{"unexpected options", func(q *domain.DraftQuestion) { q.Options = []domain.DraftOption{{Text: "A", IsCorrect: true}} }},
		{"empty fill blank", func(q *domain.DraftQuestion) { q.Type = "fill_blank" }},
		{"whitespace answer", func(q *domain.DraftQuestion) {
			q.Type = "fill_blank"
			q.Prompt = "{{1}}"
			q.Blanks = []domain.DraftBlank{{Ordinal: 1, AcceptedAnswers: []string{" "}}}
		}},
		{"duplicate slots", func(q *domain.DraftQuestion) {
			q.Type = "fill_blank"
			q.Prompt = "{{1}}"
			q.Blanks = []domain.DraftBlank{{Ordinal: 1, AcceptedAnswers: []string{"a"}}, {Ordinal: 1, AcceptedAnswers: []string{"b"}}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			question := base()
			tc.change(&question)
			if err := domain.Publishing.Validate(draftWith(question)); err == nil {
				t.Fatal("publication bypassed question validation")
			}
		})
	}
}

func TestPublicationRequiresQuestionsAndBoundedExactTotal(t *testing.T) {
	if err := domain.Publishing.Validate(domain.DraftContent{}); err == nil {
		t.Fatal("empty exam accepted")
	}
	question := base()
	question.Points = "500000"
	draft := draftWith(question)
	draft.Sections[0].Questions = append(draft.Sections[0].Questions, question)
	if v := onlyViolation(t, draft); v.Rule != domain.TotalPointsValid {
		t.Fatalf("overflow did not name total: %v", v)
	}
	draft.Sections[0].Questions[0].Points = "0.29"
	draft.Sections[0].Questions[1].Points = "0.01"
	if total, count := domain.Publishing.Totals(draft); total != "0.30" || count != 2 {
		t.Fatalf("imprecise total: %s/%d", total, count)
	}
}
