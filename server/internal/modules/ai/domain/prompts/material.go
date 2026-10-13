package prompts

import (
	"fmt"
	"strings"

	"quizzivy/internal/modules/ai/domain"
)

// MaxReadChars is the most characters of questions one check or variant reads; a longer draft is ErrTooLarge.
const MaxReadChars = 120_000

func paperText(questions []domain.PaperQuestion) (string, error) {
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
	var b strings.Builder
	for _, p := range order {
		fmt.Fprintf(&b, "Shared material %s:\n\"\"\"\n%s\n\"\"\"\n\n", passages[p], clip(p, MaxSource))
	}
	b.WriteString("Questions:\n")
	for _, q := range questions {
		fmt.Fprintf(&b, "\nQ%d [%s]", q.Number, q.Type)
		if q.Passage != "" {
			fmt.Fprintf(&b, " (uses %s)", passages[q.Passage])
		}
		fmt.Fprintf(&b, "\nPrompt: \"\"\"%s\"\"\"\n", clip(q.Prompt, 1500))
		for i, o := range q.Options {
			fmt.Fprintf(&b, "%s. %s", letter(i), clip(oneLine(o.Text), 300))
			if o.Correct {
				b.WriteString(" (correct)")
			}
			b.WriteString("\n")
		}
		for i, bl := range q.Blanks {
			fmt.Fprintf(&b, "Blank %d accepts: %s\n", i+1, strings.Join(bl.Accepted, " | "))
		}
		if q.Sample != "" {
			fmt.Fprintf(&b, "Sample answer: %s\n", clip(q.Sample, 300))
		}
		if q.Explanation != "" {
			fmt.Fprintf(&b, "Explanation: %s\n", clip(oneLine(q.Explanation), 600))
		}
	}
	if b.Len() > MaxReadChars {
		return "", domain.ErrTooLarge
	}
	return b.String(), nil
}

func questionByNumber(questions []domain.PaperQuestion, number int) (domain.PaperQuestion, bool) {
	for _, q := range questions {
		if q.Number == number {
			return q, true
		}
	}
	return domain.PaperQuestion{}, false
}
