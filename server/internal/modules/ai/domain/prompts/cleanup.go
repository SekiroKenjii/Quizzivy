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

// MaxCleanupChars is the most characters of questions one clean-up reads, which keeps a call under the provider's long-prompt price step; FitCleanup cuts a longer bank to what fits.
const MaxCleanupChars = 300_000

// MaxTagsAsked is how many of the questions marked tags=none one clean-up asks tags for, the first ones listed, so the answer fits the tokens it is given however large the bank. A bank with more says so in its answer's truncated, and a second run, after the first tags are applied, reaches the next.
const MaxTagsAsked = 60

// MaxDuplicatesAsked is how many pairs of near-duplicates one clean-up asks for.
const MaxDuplicatesAsked = 20

const cleanupMaxTokens = 3000

const cleanupHeader = "Questions (reference, type, level, tags, prompt):\n"

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
	b.WriteString(cleanupHeader)
	for i, q := range in.Questions {
		b.WriteString(cleanupLine(i, q))
		if b.Len() > MaxCleanupChars {
			return Prompt{}, domain.ErrTooLarge
		}
	}
	var what []string
	if slices.Contains(in.Checks, CleanupDuplicates) {
		what = append(what, fmt.Sprintf("duplicates = at most %d pairs of questions that test the same thing in nearly the same words, the clearest first (report each pair once, with a reason)", MaxDuplicatesAsked))
	}
	if slices.Contains(in.Checks, CleanupTags) {
		what = append(what, fmt.Sprintf("tags = for at most %d of the questions marked tags=none, the first ones listed, 1-4 short lower-case tags and, if it shows level=?, its CEFR level (pre_a1, a1, a2, b1, b2, c1 or c2)", MaxTagsAsked))
	}
	fmt.Fprintf(&b, "\nFind: %s.\nWrite the reasons in %s. Never mention a question that is fine; use [] for what you did not look for. For a question you tag, level is \"\" if you cannot tell.\n"+
		"Return {\"duplicates\":[{\"a\":\"q1\",\"b\":\"q2\",\"why\":\"...\"}],\"tags\":[{\"question\":\"q3\",\"tags\":[\"...\"],\"level\":\"b1\"}]}.",
		strings.Join(what, "; "), in.Language.Name())
	return Prompt{Feature: domain.FeatureCleanup, Tier: domain.TierFast, System: cleanupSystem, User: b.String(), Schema: json.RawMessage(cleanupSchema), MaxTokens: cleanupMaxTokens}, nil
}

func cleanupLine(i int, q domain.BankQuestion) string {
	level, tags := q.Level, strings.Join(q.Tags, ",")
	if level == "" {
		level = "?"
	}
	if tags == "" {
		tags = "none"
	}
	return fmt.Sprintf("%s [%s] level=%s tags=%s: %s\n", ref(i), q.Type, level, tags, quotedPrompt(q.Prompt))
}

// FitCleanup returns the longest run of questions, from the first, whose lines fit MaxCleanupChars, and whether any were cut.
func FitCleanup(questions []domain.BankQuestion) ([]domain.BankQuestion, bool) {
	size := len(cleanupHeader)
	for i, q := range questions {
		size += len(cleanupLine(i, q))
		if size > MaxCleanupChars {
			return questions[:i], true
		}
	}
	return questions, false
}

// Capped reports whether the input holds more questions without tags than one clean-up asks tags for, when it asks for tags at all.
func (in CleanupInput) Capped() bool {
	return slices.Contains(in.Checks, CleanupTags) && len(untagged(in.Questions)) > MaxTagsAsked
}

func untagged(questions []domain.BankQuestion) []int {
	var out []int
	for i, q := range questions {
		if len(q.Tags) == 0 {
			out = append(out, i)
		}
	}
	return out
}

func askedTags(questions []domain.BankQuestion) map[int]bool {
	out := map[int]bool{}
	for _, i := range untagged(questions) {
		if len(out) == MaxTagsAsked {
			break
		}
		out[i] = true
	}
	return out
}

func quotedPrompt(s string) string {
	return `"` + clip(oneLine(strings.ReplaceAll(s, `"`, "'")), 160) + `"`
}

// DecodeCleanup reads the provider's answer. A reference that names no question, a pair of one question, a repeated pair and a tag suggestion for a question that has tags or was not among the first MaxTagsAsked without are dropped, as is whatever was not asked for; tags are lower case, at most four, and a level is kept only for a question with none.
func DecodeCleanup(raw json.RawMessage, in CleanupInput) (CleanupFindings, error) {
	var answer struct {
		Duplicates []rawDuplicate `json:"duplicates"`
		Tags       []rawTag       `json:"tags"`
	}
	if err := decode(raw, &answer); err != nil {
		return CleanupFindings{}, err
	}
	out := CleanupFindings{Duplicates: []Duplicate{}, Tags: []TagSuggestion{}}
	if slices.Contains(in.Checks, CleanupDuplicates) {
		out.Duplicates = keptDuplicates(answer.Duplicates, in.Questions)
	}
	if slices.Contains(in.Checks, CleanupTags) {
		out.Tags = keptTags(answer.Tags, in.Questions)
	}
	return out, nil
}

type rawDuplicate struct {
	A   string `json:"a"`
	B   string `json:"b"`
	Why string `json:"why"`
}

type rawTag struct {
	Question string   `json:"question"`
	Tags     []string `json:"tags"`
	Level    string   `json:"level"`
}

func questionAt(questions []domain.BankQuestion, reference string) (int, bool) {
	want := strings.ToLower(strings.TrimSpace(reference))
	for i := range questions {
		if ref(i) == want {
			return i, true
		}
	}
	return 0, false
}

func keptDuplicates(rows []rawDuplicate, questions []domain.BankQuestion) []Duplicate {
	out := []Duplicate{}
	pairs := map[[2]int]bool{}
	for _, d := range rows {
		if len(out) == MaxDuplicates {
			break
		}
		a, okA := questionAt(questions, d.A)
		b, okB := questionAt(questions, d.B)
		why := clip(oneLine(d.Why), 300)
		pair := [2]int{min(a, b), max(a, b)}
		if !okA || !okB || a == b || why == "" || pairs[pair] {
			continue
		}
		pairs[pair] = true
		out = append(out, Duplicate{A: questions[pair[0]], B: questions[pair[1]], Why: why})
	}
	return out
}

func keptTags(rows []rawTag, questions []domain.BankQuestion) []TagSuggestion {
	out := []TagSuggestion{}
	tagged := map[int]bool{}
	asked := askedTags(questions)
	for _, t := range rows {
		if len(out) == MaxTagSuggestions {
			break
		}
		i, ok := questionAt(questions, t.Question)
		if !ok || !asked[i] || tagged[i] {
			continue
		}
		tags := lowered(distinct(t.Tags, 4, 40))
		if len(tags) == 0 {
			continue
		}
		tagged[i] = true
		out = append(out, TagSuggestion{Question: questions[i], Tags: tags, Level: suggestedLevel(t.Level, questions[i])})
	}
	return out
}

func suggestedLevel(level string, q domain.BankQuestion) string {
	level = strings.ToLower(strings.TrimSpace(level))
	if q.Level == "" && slices.Contains(levels, level) {
		return level
	}
	return ""
}

func lowered(texts []string) []string {
	out := make([]string, len(texts))
	for i, t := range texts {
		out[i] = strings.ToLower(t)
	}
	return out
}
