package prompts

import (
	"encoding/json"
	"fmt"
	"strings"

	"quizzivy/internal/modules/ai/domain"
)

// ExplanationInput is what "Write an explanation" sends: the stored question with its key, the language to write in and whether to write one sentence.
type ExplanationInput struct {
	Question domain.Question
	Language domain.Language
	Short    bool
}

const explanationSystem = "You write short explanations for test answers. Students read them after results are released. " + untrusted + "Reply with JSON only."

const explanationSchema = `{"type":"object","additionalProperties":false,"required":["explanation"],"properties":{"explanation":{"type":"string"}}}`

func isChoice(t string) bool {
	return t == SingleChoice || t == MultipleChoice || t == TrueFalse || t == Tfng
}

// KeyText is the answer key of a question in words: the correct options, the accepted answers of each blank, each matching item with the option that answers it, or the sample answer (an essay's is what a strong answer covers). It is empty for a question with none, and for a matching question with an item that names no option.
func KeyText(q domain.Question) string {
	switch {
	case q.Type == Matching:
		return matchingKeyText(q)
	case isChoice(q.Type):
		var correct []string
		for _, o := range q.Options {
			if o.Correct {
				correct = append(correct, clip(oneLine(o.Text), 300))
			}
		}
		return strings.Join(correct, "; ")
	case q.Type == FillBlank:
		var blanks []string
		for i, b := range q.Blanks {
			blanks = append(blanks, fmt.Sprintf("[%d] %s", i+1, strings.Join(b.Accepted, " / ")))
		}
		return strings.Join(blanks, "; ")
	}
	return clip(q.Sample, 300)
}

func matchingKeyText(q domain.Question) string {
	if len(q.MatchItems) == 0 {
		return ""
	}
	pairs := make([]string, len(q.MatchItems))
	for i, item := range q.MatchItems {
		if item.Option < 0 || item.Option >= len(q.Options) {
			return ""
		}
		pairs[i] = fmt.Sprintf("%d. %s -> %s. %s", i+1, clip(oneLine(item.Text), 200), letter(item.Option), clip(oneLine(q.Options[item.Option].Text), 200))
	}
	return strings.Join(pairs, "; ")
}

// Explanation builds the request for "Write an explanation". It answers domain.ErrNoKey for a question with no answer key.
func Explanation(in ExplanationInput) (Prompt, error) {
	key := KeyText(in.Question)
	if key == "" {
		return Prompt{}, domain.ErrNoKey
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Question type: %s\nQuestion: \"\"\"%s\"\"\"\n", in.Question.Type, clip(in.Question.Prompt, 1500))
	if len(in.Question.Options) > 0 {
		b.WriteString("Options:\n")
		for i, o := range in.Question.Options {
			fmt.Fprintf(&b, "%s. %s\n", letter(i), clip(oneLine(o.Text), 300))
		}
	}
	if in.Question.Type == Essay {
		fmt.Fprintf(&b, "What a strong answer covers: %s\n\n", key)
	} else {
		fmt.Fprintf(&b, "Correct answer: %s\n\n", key)
	}
	length := "in 2-3 sentences"
	if in.Short {
		length = "in one sentence"
	}
	fmt.Fprintf(&b, "Write the explanation in %s, %s. Say why the correct answer is right", in.Language.Name(), length)
	switch {
	case in.Question.Type == Matching:
		b.WriteString(", item by item, briefly")
	case in.Question.Type == Essay:
		b.WriteString(": what a strong answer covers and how it is organised")
	case len(in.Question.Options) > 1:
		b.WriteString(" and why the most tempting wrong option is wrong")
	}
	b.WriteString(". Use plain words a B1 learner understands. Put the explanation in \"explanation\".")
	return Prompt{Feature: domain.FeatureExplanation, Tier: domain.TierCapable, System: explanationSystem, User: b.String(), Schema: json.RawMessage(explanationSchema), MaxTokens: 600}, nil
}

// DecodeExplanation reads the provider's answer: the explanation, cut to 2,000 characters. An empty one is ErrUnusable.
func DecodeExplanation(raw json.RawMessage) (string, error) {
	var answer struct {
		Explanation string `json:"explanation"`
	}
	if err := decode(raw, &answer); err != nil {
		return "", err
	}
	text := clip(answer.Explanation, 2000)
	if text == "" {
		return "", ErrUnusable
	}
	return text, nil
}
