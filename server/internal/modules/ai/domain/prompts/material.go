package prompts

import (
	"fmt"
	"strings"

	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/shared/matchkey"
)

// MaxReadChars is the most characters of questions one check or variant reads; a longer draft is ErrTooLarge.
const MaxReadChars = 120_000

func paperText(questions []domain.PaperQuestion) (string, error) {
	passages, order := passageIndex(questions)
	var b strings.Builder
	for _, p := range order {
		fmt.Fprintf(&b, "Shared material %s:\n\"\"\"\n%s\n\"\"\"\n\n", passages[p], clip(p, MaxSource))
	}
	b.WriteString("Questions:\n")
	for _, q := range questions {
		writeQuestion(&b, q, passages)
	}
	if b.Len() > MaxReadChars {
		return "", domain.ErrTooLarge
	}
	return b.String(), nil
}

func passageIndex(questions []domain.PaperQuestion) (map[string]string, []string) {
	passages := map[string]string{}
	var order []string
	for _, q := range questions {
		if q.Passage == "" {
			continue
		}
		if _, seen := passages[q.Passage]; !seen {
			passages[q.Passage] = fmt.Sprintf("P%d", len(order)+1)
			order = append(order, q.Passage)
		}
	}
	return passages, order
}

func writeQuestion(b *strings.Builder, q domain.PaperQuestion, passages map[string]string) {
	fmt.Fprintf(b, "\nQ%d [%s]", q.Number, q.Type)
	if q.Passage != "" {
		fmt.Fprintf(b, " (uses %s)", passages[q.Passage])
	}
	fmt.Fprintf(b, "\nPrompt: \"\"\"%s\"\"\"\n", clip(q.Prompt, 1500))
	for i, o := range q.Options {
		fmt.Fprintf(b, "%s. %s", optionLabel(q.Type, i), clip(oneLine(o.Text), 300))
		if o.Correct {
			b.WriteString(" (correct)")
		}
		b.WriteString("\n")
	}
	if q.SelectCount > 0 {
		fmt.Fprintf(b, "Choose exactly %d options\n", q.SelectCount)
	}
	for i, item := range q.MatchItems {
		fmt.Fprintf(b, "Item %s. %s Answer: %s\n", matchkey.Item(i), clip(oneLine(item.Text), 300), matchKey(item, len(q.Options)))
	}
	for i, bl := range q.Blanks {
		fmt.Fprintf(b, "Blank %d accepts: %s\n", i+1, strings.Join(bl.Accepted, " | "))
	}
	if q.Sample != "" {
		fmt.Fprintf(b, "Sample answer: %s\n", clip(q.Sample, 300))
	}
	if len(q.Criteria) > 0 {
		labels := make([]string, len(q.Criteria))
		for i, c := range q.Criteria {
			labels[i] = clip(oneLine(c.Label), 80)
		}
		fmt.Fprintf(b, "Rubric criteria: %s\n", strings.Join(labels, " | "))
	}
	switch {
	case q.MaxWords > 0:
		fmt.Fprintf(b, "Word limit: %d\n", q.MaxWords)
	case q.Type == ShortAnswer:
		b.WriteString("Word limit: none\n")
	}
	if q.Explanation != "" {
		fmt.Fprintf(b, "Explanation: %s\n", clip(oneLine(q.Explanation), 600))
	}
}

// matchKey names the choice a matching item is keyed to by its D-063 key, or says it has none.
func matchKey(item domain.MatchItem, options int) string {
	if item.Option < 0 || item.Option >= options {
		return "(no answer)"
	}
	return matchkey.Option(item.Option)
}

// optionLabel is how the assistant is shown an option: a matching choice by its D-063 key (i, ii, iii), any other option by its letter.
func optionLabel(questionType string, i int) string {
	if questionType == Matching {
		return matchkey.Option(i)
	}
	return letter(i)
}

// optionAt reads the option a label names: a matching choice by its D-063 key, any other option by its letter. It is -1 for a label that names none.
func optionAt(label string, q domain.PaperQuestion) int {
	if q.Type != Matching {
		return letterIndex(label)
	}
	label = strings.ToLower(strings.TrimSpace(label))
	for i := range q.Options {
		if matchkey.Option(i) == label {
			return i
		}
	}
	return -1
}

func questionByNumber(questions []domain.PaperQuestion, number int) (domain.PaperQuestion, bool) {
	for _, q := range questions {
		if q.Number == number {
			return q, true
		}
	}
	return domain.PaperQuestion{}, false
}
