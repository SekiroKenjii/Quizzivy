package prompts

import (
	"encoding/json"
	"fmt"
	"strings"

	"quizzivy/internal/modules/ai/domain"
)

// MaxSource is how many characters of a source are sent.
const MaxSource = 8000

// GenerateInput is what "Generate questions" sends: the source text the teacher pasted or picked, the settings, and the language explanations are written in. Level is a CEFR level in lower case.
type GenerateInput struct {
	Source       string
	Types        []string
	Count        int
	Level        string
	Explanations bool
	Language     domain.Language
}

// Question is a draft question the assistant wrote. For a choice type Options are in order and AnswerKey holds the text of each correct one exactly as written in Options; a true/false question has the options True and False; for fill_blank Prompt holds one ___ and AnswerKey the accepted answers; for short_answer AnswerKey holds the sample answers.
type Question struct {
	Type        string
	Prompt      string
	Options     []string
	AnswerKey   []string
	Explanation string
}

const generateSystem = "You write English test questions for Vietnamese learners. Every answer must be supported by the source. " + untrusted + "Reply with JSON only."

const generateSchema = `{"type":"object","additionalProperties":false,"required":["questions"],"properties":{"questions":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["type","prompt","options","answers","explanation"],"properties":{"type":{"type":"string","enum":["single_choice","multiple_choice","true_false","fill_blank","short_answer"]},"prompt":{"type":"string"},"options":{"type":"array","items":{"type":"string"}},"answers":{"type":"array","items":{"type":"string"}},"explanation":{"type":"string"}}}}}}`

var typeRules = map[string]string{
	SingleChoice:   "single_choice = 4 options, one correct.",
	MultipleChoice: "multiple_choice = 5 options, two or three correct.",
	TrueFalse:      "true_false = a statement; the options are exactly True and False.",
	FillBlank:      "fill_blank = a sentence with ___ for one gap; the answer is 1-3 words from the source.",
	ShortAnswer:    "short_answer = a question answered in 1-3 words from the source.",
}

// Generate builds the request for "Generate questions". The source is cut to MaxSource characters.
func Generate(in GenerateInput) Prompt {
	var rules []string
	for _, t := range in.Types {
		rules = append(rules, typeRules[t])
	}
	explain := "Leave every explanation empty."
	if in.Explanations {
		explain = fmt.Sprintf("Write each explanation in %s: 1-2 sentences saying where the source supports the answer.", in.Language.Name())
	}
	user := fmt.Sprintf("Source:\n\"\"\"\n%s\n\"\"\"\n\nWrite %d questions at CEFR %s. Use only these types: %s.\n%s\n"+
		"For single_choice, multiple_choice and true_false, answers must equal option texts exactly. For fill_blank and short_answer, options is an empty array.\n%s\n"+
		"Return {\"questions\":[{\"type\":\"...\",\"prompt\":\"...\",\"options\":[\"...\"],\"answers\":[\"...\"],\"explanation\":\"...\"}]}.",
		clip(in.Source, MaxSource), in.Count, strings.ToUpper(in.Level), strings.Join(in.Types, ", "), strings.Join(rules, "\n"), explain)
	return Prompt{Feature: domain.FeatureGenerate, Tier: domain.TierCapable, System: generateSystem, User: user, Schema: json.RawMessage(generateSchema), MaxTokens: 6000}
}

type rawQuestion struct {
	Type        string   `json:"type"`
	Prompt      string   `json:"prompt"`
	Options     []string `json:"options"`
	Answers     []string `json:"answers"`
	Explanation string   `json:"explanation"`
}

// DecodeGenerate reads the provider's answer. A question of a type that was not asked for, with an answer that is not one of its options, or with a gap that is not exactly one is dropped, and no more than in.Count are kept; if none is left the answer is ErrUnusable.
func DecodeGenerate(raw json.RawMessage, in GenerateInput) ([]Question, error) {
	var answer struct {
		Questions []rawQuestion `json:"questions"`
	}
	if err := decode(raw, &answer); err != nil {
		return nil, err
	}
	var out []Question
	for _, q := range answer.Questions {
		if len(out) == in.Count {
			break
		}
		kept, ok := tidyQuestion(q, in)
		if ok {
			out = append(out, kept)
		}
	}
	if len(out) == 0 {
		return nil, ErrUnusable
	}
	return out, nil
}

func tidyQuestion(q rawQuestion, in GenerateInput) (Question, bool) {
	asked := false
	for _, t := range in.Types {
		asked = asked || t == q.Type
	}
	prompt := strings.TrimSpace(q.Prompt)
	if !asked || prompt == "" || len([]rune(prompt)) > 2000 {
		return Question{}, false
	}
	out := Question{Type: q.Type, Prompt: prompt}
	if in.Explanations {
		out.Explanation = clip(q.Explanation, 2000)
	}
	switch q.Type {
	case SingleChoice, MultipleChoice:
		out.Options = distinct(q.Options, 8, 500)
		out.AnswerKey = matchingOptions(out.Options, q.Answers)
		if len(out.Options) < 2 || len(out.AnswerKey) == 0 || len(out.AnswerKey) >= len(out.Options) {
			return Question{}, false
		}
		if q.Type == SingleChoice && len(out.AnswerKey) != 1 {
			return Question{}, false
		}
	case TrueFalse:
		out.Options = []string{"True", "False"}
		out.AnswerKey = matchingOptions(out.Options, q.Answers)
		if len(out.AnswerKey) != 1 {
			return Question{}, false
		}
	case FillBlank:
		out.AnswerKey = distinct(q.Answers, 8, 100)
		if strings.Count(prompt, "___") != 1 || len(out.AnswerKey) == 0 {
			return Question{}, false
		}
	case ShortAnswer:
		out.AnswerKey = distinct(q.Answers, 8, 200)
		if len(out.AnswerKey) == 0 {
			return Question{}, false
		}
	}
	return out, true
}

func matchingOptions(options, answers []string) []string {
	var out []string
	for _, answer := range answers {
		for _, option := range options {
			if sameText(option, answer) && !contains(out, option) {
				out = append(out, option)
			}
		}
	}
	return out
}

func contains(texts []string, text string) bool {
	for _, t := range texts {
		if t == text {
			return true
		}
	}
	return false
}
