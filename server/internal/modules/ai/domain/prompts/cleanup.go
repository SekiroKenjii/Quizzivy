package prompts

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"quizzivy/internal/modules/ai/domain"
)

// The checks a bank clean-up may be asked for.
const (
	CleanupDuplicates = "duplicates"
	CleanupTags       = "tags"
)

// MaxCleanupChars is the most characters of questions one clean-up reads; a longer bank is ErrTooLarge.
const MaxCleanupChars = 600_000

// CleanupInput is what "Clean up the bank" sends: the prompts, types, levels and tags of the caller's own questions, each under a short reference the answer uses, so no id is sent. Language is the language the reasons are written in.
type CleanupInput struct {
	Questions []domain.BankQuestion
	Checks    []string
	Language  domain.Language
}

// Duplicate is a pair of questions that test the same thing in nearly the same words.
type Duplicate struct {
	A, B domain.BankQuestion
	Why  string
}

// TagSuggestion is the tags, and the CEFR level when the question has none, proposed for a question without tags.
type TagSuggestion struct {
	Question domain.BankQuestion
	Level    string
	Tags     []string
}

// CleanupFindings is what a clean-up found.
type CleanupFindings struct {
	Duplicates []Duplicate
	Tags       []TagSuggestion
}

// MaxDuplicates and MaxTagSuggestions bound what one clean-up returns.
const (
	MaxDuplicates     = 50
	MaxTagSuggestions = 200
)

const cleanupSystem = "You tidy a teacher's English question bank. " + untrusted + "Reply with JSON only."

const cleanupSchema = `{"type":"object","additionalProperties":false,"required":["duplicates","tags"],"properties":{"duplicates":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["a","b","why"],"properties":{"a":{"type":"string"},"b":{"type":"string"},"why":{"type":"string"}}}},"tags":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["question","tags","level"],"properties":{"question":{"type":"string"},"tags":{"type":"array","items":{"type":"string"}},"level":{"type":"string"}}}}}}`

var levels = []string{"pre_a1", "a1", "a2", "b1", "b2", "c1", "c2"}

func ref(i int) string { return fmt.Sprintf("q%d", i+1) }

// Cleanup builds the request for "Clean up the bank". It answers domain.ErrNothingToRead for a bank with no question and domain.ErrTooLarge for one longer than MaxCleanupChars.
func Cleanup(in CleanupInput) (Prompt, error) {
	if len(in.Questions) == 0 {
		return Prompt{}, domain.ErrNothingToRead
	}
	var b strings.Builder
	b.WriteString("Questions (reference, type, level, tags, prompt):\n")
	for i, q := range in.Questions {
		level, tags := q.Level, strings.Join(q.Tags, ",")
		if level == "" {
			level = "?"
		}
		if tags == "" {
			tags = "none"
		}
		fmt.Fprintf(&b, "%s [%s] level=%s tags=%s: %s\n", ref(i), q.Type, level, tags, quotedPrompt(q.Prompt))
		if b.Len() > MaxCleanupChars {
			return Prompt{}, domain.ErrTooLarge
		}
	}
	var what []string
	if slices.Contains(in.Checks, CleanupDuplicates) {
		what = append(what, "duplicates = pairs of questions that test the same thing in nearly the same words (report each pair once, with a reason)")
	}
	if slices.Contains(in.Checks, CleanupTags) {
		what = append(what, "tags = for each question marked tags=none, 1-4 short lower-case tags and, if it shows level=?, its CEFR level (pre_a1, a1, a2, b1, b2, c1 or c2)")
	}
	fmt.Fprintf(&b, "\nFind: %s.\nWrite the reasons in %s. Never mention a question that is fine; use [] for what you did not look for. For a question you tag, level is \"\" if you cannot tell.\n"+
		"Return {\"duplicates\":[{\"a\":\"q1\",\"b\":\"q2\",\"why\":\"...\"}],\"tags\":[{\"question\":\"q3\",\"tags\":[\"...\"],\"level\":\"b1\"}]}.",
		strings.Join(what, "; "), in.Language.Name())
	return Prompt{Feature: domain.FeatureCleanup, Tier: domain.TierFast, System: cleanupSystem, User: b.String(), Schema: json.RawMessage(cleanupSchema), MaxTokens: 8000}, nil
}

func quotedPrompt(s string) string {
	return `"` + clip(oneLine(strings.ReplaceAll(s, `"`, "'")), 160) + `"`
}

// DecodeCleanup reads the provider's answer. A reference that names no question, a pair of one question, a repeated pair and a tag suggestion for a question that has tags are dropped, as is whatever was not asked for; tags are lower case, at most four, and a level is kept only for a question with none.
func DecodeCleanup(raw json.RawMessage, in CleanupInput) (CleanupFindings, error) {
	var answer struct {
		Duplicates []struct {
			A   string `json:"a"`
			B   string `json:"b"`
			Why string `json:"why"`
		} `json:"duplicates"`
		Tags []struct {
			Question string   `json:"question"`
			Tags     []string `json:"tags"`
			Level    string   `json:"level"`
		} `json:"tags"`
	}
	if err := decode(raw, &answer); err != nil {
		return CleanupFindings{}, err
	}
	at := func(r string) (int, bool) {
		for i := range in.Questions {
			if ref(i) == strings.ToLower(strings.TrimSpace(r)) {
				return i, true
			}
		}
		return 0, false
	}
	out := CleanupFindings{Duplicates: []Duplicate{}, Tags: []TagSuggestion{}}
	if slices.Contains(in.Checks, CleanupDuplicates) {
		pairs := map[[2]int]bool{}
		for _, d := range answer.Duplicates {
			a, okA := at(d.A)
			b, okB := at(d.B)
			why := clip(oneLine(d.Why), 300)
			if !okA || !okB || a == b || why == "" || len(out.Duplicates) == MaxDuplicates {
				continue
			}
			pair := [2]int{min(a, b), max(a, b)}
			if pairs[pair] {
				continue
			}
			pairs[pair] = true
			out.Duplicates = append(out.Duplicates, Duplicate{A: in.Questions[pair[0]], B: in.Questions[pair[1]], Why: why})
		}
	}
	if slices.Contains(in.Checks, CleanupTags) {
		tagged := map[int]bool{}
		for _, t := range answer.Tags {
			i, ok := at(t.Question)
			if !ok || len(in.Questions[i].Tags) > 0 || tagged[i] || len(out.Tags) == MaxTagSuggestions {
				continue
			}
			tags := lowered(distinct(t.Tags, 4, 40))
			if len(tags) == 0 {
				continue
			}
			tagged[i] = true
			suggestion := TagSuggestion{Question: in.Questions[i], Tags: tags}
			if level := strings.ToLower(strings.TrimSpace(t.Level)); in.Questions[i].Level == "" && slices.Contains(levels, level) {
				suggestion.Level = level
			}
			out.Tags = append(out.Tags, suggestion)
		}
	}
	return out, nil
}

func lowered(texts []string) []string {
	out := make([]string, len(texts))
	for i, t := range texts {
		out[i] = strings.ToLower(t)
	}
	return out
}
