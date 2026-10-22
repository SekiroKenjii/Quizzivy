package prompts

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"quizzivy/internal/modules/ai/domain"
)

// MaxSource is how many characters of a source are sent.
const MaxSource = 8000

// GenerateInput is what "Generate questions" sends: the source text the teacher pasted or picked, the settings, and the language explanations are written in. Level is a CEFR level in lower case. Standalone asks for questions that each stand alone, so Source is a topic or notes they are about rather than a text they are answered from, and may be empty when Section is named; Section is the section they are written for, whose existing questions are not to be repeated.
type GenerateInput struct {
	Source       string
	Types        []string
	Count        int
	Level        string
	Explanations bool
	Language     domain.Language
	Standalone   bool
	Section      *domain.Section
}

// Question is a draft question the assistant wrote. For a choice type Options are in order and AnswerKey holds the text of each correct one exactly as written in Options; a true/false question has the options True and False, and a tfng question True, False and Not given; for fill_blank Prompt holds one ___ and AnswerKey the accepted answers; for short_answer AnswerKey holds the sample answers.
type Question struct {
	Type        string
	Prompt      string
	Options     []string
	AnswerKey   []string
	Explanation string
}

const generateSystem = "You write English test questions for Vietnamese learners. Every answer must be supported by the source. " + untrusted + "Reply with JSON only."

const standaloneSystem = "You write English test questions for Vietnamese learners. Each question stands alone: it holds everything needed to answer it and never refers to a passage, a text, a recording, a conversation, a picture or another question. Each key is the one answer a careful teacher would accept. " + untrusted + "Reply with JSON only."

const generateSchema = `{"type":"object","additionalProperties":false,"required":["questions"],"properties":{"questions":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["type","prompt","options","answers","explanation"],"properties":{"type":{"type":"string","enum":["single_choice","multiple_choice","true_false","fill_blank","short_answer","tfng"]},"prompt":{"type":"string"},"options":{"type":"array","items":{"type":"string"}},"answers":{"type":"array","items":{"type":"string"}},"explanation":{"type":"string"}}}}}}`

var typeRules = map[string]string{
	SingleChoice:   "single_choice = 4 options, one correct.",
	MultipleChoice: "multiple_choice = 5 options, two or three correct.",
	TrueFalse:      "true_false = a statement; the options are exactly True and False.",
	FillBlank:      "fill_blank = a sentence with ___ for one gap; the answer is 1-3 words from the source.",
	ShortAnswer:    "short_answer = a question answered in 1-3 words from the source.",
	Tfng:           "tfng = a statement about the source; the options are exactly True, False and Not given. True when the source says it, False when the source says the opposite, Not given when the source does not say either way.",
}

var standaloneRules = map[string]string{
	SingleChoice:   typeRules[SingleChoice],
	MultipleChoice: typeRules[MultipleChoice],
	TrueFalse:      typeRules[TrueFalse],
	FillBlank:      "fill_blank = a sentence with ___ for one gap that only one answer of 1-3 words fits; list the accepted spellings.",
	ShortAnswer:    "short_answer = a question with one answer of 1-3 words.",
}

// Generate builds the request for "Generate questions". The source is cut to MaxSource characters, and so is a section's text.
func Generate(in GenerateInput) Prompt {
	if in.Standalone {
		return standalone(in)
	}
	var rules []string
	for _, t := range in.Types {
		rules = append(rules, typeRules[t])
	}
	explain := "Leave every explanation empty."
	if in.Explanations {
		explain = fmt.Sprintf("Write each explanation in %s: 1-2 sentences saying where the source supports the answer.", in.Language.Name())
	}
	user := fmt.Sprintf("Source:\n\"\"\"\n%s\n\"\"\"\n\nWrite %d questions at CEFR %s. Use only these types: %s.\n%s\n"+
		"For single_choice, multiple_choice, true_false and tfng, answers must equal option texts exactly. For fill_blank and short_answer, options is an empty array.\n%s\n"+
		"Put all %d questions in \"questions\", each with its type, prompt, options, answers and explanation written from the source.",
		clip(in.Source, MaxSource), in.Count, strings.ToUpper(in.Level), strings.Join(in.Types, ", "), strings.Join(rules, "\n"), explain, in.Count)
	return Prompt{Feature: domain.FeatureGenerate, Tier: domain.TierCapable, System: generateSystem, User: user, Schema: json.RawMessage(generateSchema), MaxTokens: 6000}
}

func standalone(in GenerateInput) Prompt {
	var rules []string
	for _, t := range in.Types {
		rules = append(rules, standaloneRules[t])
	}
	explain := "Leave every explanation empty."
	if in.Explanations {
		explain = fmt.Sprintf("Write each explanation in %s: 1-2 sentences saying why the answer is right.", in.Language.Name())
	}
	var parts []string
	if topic := clip(in.Source, MaxSource); topic != "" {
		parts = append(parts, fmt.Sprintf("Topic or notes from the teacher:\n\"\"\"\n%s\n\"\"\"", topic))
	}
	if in.Section != nil {
		parts = append(parts, sectionBlock(*in.Section))
	}
	ask := fmt.Sprintf("Write %d new questions at CEFR %s. Use only these types: %s.\n%s\n"+
		"For single_choice, multiple_choice and true_false, answers must equal option texts exactly. For fill_blank and short_answer, options is an empty array.\n%s\n"+
		"Put all %d questions in \"questions\", each with its type, prompt, options, answers and explanation.",
		in.Count, strings.ToUpper(in.Level), strings.Join(in.Types, ", "), strings.Join(rules, "\n"), explain, in.Count)
	if in.Section != nil {
		ask = "Match the section's skill, style and difficulty, and do not repeat or reword a question already in it.\n" + ask
	}
	parts = append(parts, ask)
	return Prompt{Feature: domain.FeatureGenerate, Tier: domain.TierCapable, System: standaloneSystem, User: strings.Join(parts, "\n\n"), Schema: json.RawMessage(generateSchema), MaxTokens: 6000}
}

func sectionBlock(section domain.Section) string {
	var b strings.Builder
	b.WriteString("The section these questions are for:\n\"\"\"\n")
	if title := oneLine(section.Title); title != "" {
		fmt.Fprintf(&b, "Title: %s\n", title)
	}
	if instructions := oneLine(section.Instructions); instructions != "" {
		fmt.Fprintf(&b, "Instructions: %s\n", instructions)
	}
	if len(section.Questions) > 0 {
		b.WriteString("Questions already in it:\n")
	}
	for i, q := range section.Questions {
		fmt.Fprintf(&b, "%d. [%s] %s\n", i+1, q.Type, clip(oneLine(q.Prompt), 400))
	}
	return clip(b.String(), MaxSource) + "\n\"\"\""
}

type rawQuestion struct {
	Type        string   `json:"type"`
	Prompt      string   `json:"prompt"`
	Options     []string `json:"options"`
	Answers     []string `json:"answers"`
	Explanation string   `json:"explanation"`
}

// DecodeGenerate reads the provider's answer. A question of a type that was not asked for, with an answer that is not one of its options, with a gap that is not exactly one, or whose prompt repeats word for word a question already in in.Section or an earlier one of the answer is dropped, and so is a standalone question that points at a passage, recording or other material it does not have; no more than in.Count are kept; if none is left the answer is ErrUnusable.
func DecodeGenerate(raw json.RawMessage, in GenerateInput) ([]Question, error) {
	var answer struct {
		Questions []rawQuestion `json:"questions"`
	}
	if err := decode(raw, &answer); err != nil {
		return nil, err
	}
	var out []Question
	seen := existingPrompts(in)
	for _, q := range answer.Questions {
		if len(out) == in.Count {
			break
		}
		kept, ok := tidyQuestion(q, in)
		if !ok || repeats(seen, kept.Prompt) || (in.Standalone && leansOnMaterial(kept.Prompt)) {
			continue
		}
		seen = append(seen, kept.Prompt)
		out = append(out, kept)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: none of the %d questions in the answer is usable", ErrUnusable, len(answer.Questions))
	}
	return out, nil
}

var leaningOnMaterial = regexp.MustCompile(`(?i)\b(according to the (passage|text|article|conversation|dialogue|recording|extract|audio|speaker)|(in|from) the (passage|text|article|conversation|dialogue|recording|extract|audio|listening|reading)\b|the (passage|extract|recording|conversation|dialogue|following text)\b|the speaker (says|said|mentions|mentioned|thinks|believes|means))`)

func existingPrompts(in GenerateInput) []string {
	if in.Section == nil {
		return nil
	}
	prompts := make([]string, 0, len(in.Section.Questions))
	for _, q := range in.Section.Questions {
		prompts = append(prompts, q.Prompt)
	}
	return prompts
}

func repeats(seen []string, prompt string) bool {
	return slices.ContainsFunc(seen, func(s string) bool { return sameText(s, prompt) })
}

func leansOnMaterial(prompt string) bool {
	return leaningOnMaterial.MatchString(prompt)
}

func tidyQuestion(q rawQuestion, in GenerateInput) (Question, bool) {
	prompt := strings.TrimSpace(q.Prompt)
	if !slices.Contains(in.Types, q.Type) || prompt == "" || len([]rune(prompt)) > 2000 {
		return Question{}, false
	}
	out := Question{Type: q.Type, Prompt: prompt}
	if in.Explanations {
		out.Explanation = clip(q.Explanation, 2000)
	}
	return withKey(out, q)
}

func withKey(out Question, q rawQuestion) (Question, bool) {
	switch q.Type {
	case SingleChoice, MultipleChoice:
		out.Options = distinct(q.Options, 8, 500)
		out.AnswerKey = matchingOptions(out.Options, q.Answers)
		return out, choiceKeyHolds(out)
	case TrueFalse:
		out.Options = []string{"True", "False"}
		out.AnswerKey = matchingOptions(out.Options, q.Answers)
		return out, len(out.AnswerKey) == 1
	case Tfng:
		out.Options = TfngOptions()
		out.AnswerKey = matchingOptions(out.Options, q.Answers)
		return out, len(out.AnswerKey) == 1
	case FillBlank:
		out.AnswerKey = distinct(q.Answers, 8, 100)
		return out, strings.Count(out.Prompt, "___") == 1 && len(out.AnswerKey) > 0
	case ShortAnswer:
		out.AnswerKey = distinct(q.Answers, 8, 200)
		return out, len(out.AnswerKey) > 0
	}
	return out, true
}

func choiceKeyHolds(q Question) bool {
	if len(q.Options) < 2 || len(q.AnswerKey) == 0 || len(q.AnswerKey) >= len(q.Options) {
		return false
	}
	return q.Type != SingleChoice || len(q.AnswerKey) == 1
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
