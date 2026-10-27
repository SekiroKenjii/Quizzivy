package prompts

import (
	"fmt"
	"strings"

	"quizzivy/internal/modules/ai/domain"
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
		fmt.Fprintf(b, "%s. %s", letter(i), clip(oneLine(o.Text), 300))
		if o.Correct {
			b.WriteString(" (correct)")
		}
		b.WriteString("\n")
	}
	if q.SelectCount > 0 {
		fmt.Fprintf(b, "Choose exactly %d options\n", q.SelectCount)
	}
	for i, item := range q.MatchItems {
		fmt.Fprintf(b, "Item %d. %s -> %s\n", i+1, clip(oneLine(item.Text), 300), matchKey(item, len(q.Options)))
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

// matchKey names the option a matching item is keyed to by its letter, or says it has none.
func matchKey(item domain.MatchItem, options int) string {
	if item.Option < 0 || item.Option >= options {
		return "(no answer)"
	}
	return letter(item.Option)
}

func questionByNumber(questions []domain.PaperQuestion, number int) (domain.PaperQuestion, bool) {
	for _, q := range questions {
		if q.Number == number {
			return q, true
		}
	}
	return domain.PaperQuestion{}, false
}
