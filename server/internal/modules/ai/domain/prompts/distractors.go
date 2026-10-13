package prompts

import (
	"encoding/json"
	"fmt"
	"strings"

	"quizzivy/internal/modules/ai/domain"
)

// DistractorsInput is what "Suggest wrong options" sends: the stored choice question with its key and options, and the language the reasons are written in.
type DistractorsInput struct {
	Question domain.Question
	Language domain.Language
}

// Distractor is a suggested wrong option and why a learner might pick it.
type Distractor struct {
	Text string
	Why  string
}

// MaxDistractors is the most wrong options one call suggests.
const MaxDistractors = 4

const distractorsSystem = "You write wrong options (distractors) for English multiple-choice questions. " + untrusted + "Reply with JSON only."

const distractorsSchema = `{"type":"object","additionalProperties":false,"required":["distractors"],"properties":{"distractors":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["text","why"],"properties":{"text":{"type":"string"},"why":{"type":"string"}}}}}}`

// Distractors builds the request for "Suggest wrong options". It answers domain.ErrNotChoice for a question that is not a single or multiple choice question and domain.ErrNoKey for one with no correct option.
func Distractors(in DistractorsInput) (Prompt, error) {
	if in.Question.Type != SingleChoice && in.Question.Type != MultipleChoice {
		return Prompt{}, domain.ErrNotChoice
	}
	key := KeyText(in.Question)
	if key == "" {
		return Prompt{}, domain.ErrNoKey
	}
	have := make([]string, len(in.Question.Options))
	for i, o := range in.Question.Options {
		have[i] = clip(oneLine(o.Text), 300)
	}
	user := fmt.Sprintf("Question: \"\"\"%s\"\"\"\nCorrect answer: %s\nExisting options: %s\n\n"+
		"Suggest %d new wrong options. Each must be clearly wrong to someone who knows the rule, but tempting for a learner. Match the length and grammar of the correct answer and do not repeat existing options. "+
		"Write why in %s, in under 12 words. Return {\"distractors\":[{\"text\":\"...\",\"why\":\"why a learner might pick it\"}]}.",
		clip(in.Question.Prompt, 1500), key, strings.Join(have, " | "), MaxDistractors, in.Language.Name())
	return Prompt{Feature: domain.FeatureDistractors, Tier: domain.TierCapable, System: distractorsSystem, User: user, Schema: json.RawMessage(distractorsSchema), MaxTokens: 700}, nil
}

// DecodeDistractors reads the provider's answer. A suggestion that repeats an existing option or an earlier suggestion is dropped, and no more than MaxDistractors are kept; if none is left the answer is ErrUnusable.
func DecodeDistractors(raw json.RawMessage, in DistractorsInput) ([]Distractor, error) {
	var answer struct {
		Distractors []struct {
			Text string `json:"text"`
			Why  string `json:"why"`
		} `json:"distractors"`
	}
	if err := decode(raw, &answer); err != nil {
		return nil, err
	}
	taken := make([]string, 0, len(in.Question.Options)+MaxDistractors)
	for _, o := range in.Question.Options {
		taken = append(taken, o.Text)
	}
	var out []Distractor
	for _, d := range answer.Distractors {
		text, why := clip(oneLine(d.Text), 500), clip(oneLine(d.Why), 200)
		if len(out) == MaxDistractors || text == "" || why == "" {
			continue
		}
		repeats := false
		for _, t := range taken {
			repeats = repeats || sameText(t, text)
		}
		if repeats {
			continue
		}
		taken = append(taken, text)
		out = append(out, Distractor{Text: text, Why: why})
	}
	if len(out) == 0 {
		return nil, ErrUnusable
	}
	return out, nil
}
