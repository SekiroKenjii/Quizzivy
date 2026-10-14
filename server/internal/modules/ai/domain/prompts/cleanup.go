package prompts

import (
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"

	"quizzivy/internal/modules/ai/domain"
)

// The checks a bank clean-up may be asked for.
const (
	CleanupDuplicates = "duplicates"
	CleanupTags       = "tags"
)

// The bounds of one clean-up. It asks tags for at most MaxTagsAsked of the questions that have none, TagChunkSize to a call, so each answer fits the tokens it is given however large the bank is, and it asks the provider to judge at most MaxCandidatePairs pairs of questions that look alike, of which it keeps at most MaxDuplicatesAsked. A call whose user turn is longer than MaxCleanupBytes is refused with domain.ErrTooLarge. A token covers at least a byte, so that keeps a call under the provider's 100,000-token price step in any language. A tagging call never reaches it (50 lines of at most about 700 bytes and a vocabulary of about 5,000, whatever the letters); a duplicates call can, on a bank of long multi-byte prompts and tags, which FitDuplicates cuts to fit.
const (
	TagChunkSize       = 50
	MaxTagChunks       = 4
	MaxTagsAsked       = TagChunkSize * MaxTagChunks
	MaxCandidatePairs  = 100
	MaxDuplicatesAsked = 20
	MaxCleanupBytes    = 100_000
)

// CleanupTier is the tier every call of a clean-up runs on.
const CleanupTier = domain.TierFast

// MaxTagSuggestions bounds the tag suggestions one clean-up returns.
const MaxTagSuggestions = 200

const (
	cleanupMaxTokens = 3000
	vocabularySize   = 30
	maxLineTags      = 4
	maxTagRunes      = 40
)

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

const tagsSystem = "You tag a teacher's English question bank. " + untrusted + "Reply with JSON only."

const duplicatesSystem = "You find near-duplicate questions in a teacher's English question bank. " + untrusted + "Reply with JSON only."

const tagsSchema = `{"type":"object","additionalProperties":false,"required":["tags"],"properties":{"tags":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["question","tags","level"],"properties":{"question":{"type":"string"},"tags":{"type":"array","items":{"type":"string"}},"level":{"type":"string"}}}}}}`

const duplicatesSchema = `{"type":"object","additionalProperties":false,"required":["duplicates"],"properties":{"duplicates":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["a","b","why"],"properties":{"a":{"type":"string"},"b":{"type":"string"},"why":{"type":"string"}}}}}}`

var levels = []string{"pre_a1", "a1", "a2", "b1", "b2", "c1", "c2"}

func ref(i int) string { return fmt.Sprintf("q%d", i+1) }

func indexOfRef(reference string, questions int) (int, bool) {
	want := strings.ToLower(strings.TrimSpace(reference))
	n, err := strconv.Atoi(strings.TrimPrefix(want, "q"))
	if err != nil || n < 1 || n > questions || ref(n-1) != want {
		return 0, false
	}
	return n - 1, true
}

// TagChunks picks the questions of bank that have no tags, the first MaxTagsAsked of them, and splits their places in bank into runs of at most TagChunkSize, one run to a call. more reports that the bank held more questions without tags than that.
func TagChunks(bank []domain.BankQuestion) (chunks [][]int, more bool) {
	var run []int
	asked := 0
	for i, q := range bank {
		if len(q.Tags) > 0 {
			continue
		}
		if asked == MaxTagsAsked {
			more = true
			break
		}
		asked++
		run = append(run, i)
		if len(run) == TagChunkSize {
			chunks = append(chunks, run)
			run = nil
		}
	}
	if len(run) > 0 {
		chunks = append(chunks, run)
	}
	return chunks, more
}

// TaggingInput is one call that proposes tags: the whole bank, which the references name (qN is Bank[N-1], so a question keeps its reference in every call of a clean-up and no id is sent), and the places in it of the questions this call is asked about.
type TaggingInput struct {
	Bank  []domain.BankQuestion
	Asked []int
}

// Tagging builds the request that proposes tags for the Asked questions. It sends their prompts, types and levels, and the tags the bank already uses, so that a suggestion reuses a tag a teacher has chosen. It answers domain.ErrNothingToRead when nothing is asked, an error for a place outside the bank, and domain.ErrTooLarge for a call longer than MaxCleanupBytes.
func Tagging(in TaggingInput) (Prompt, error) {
	if len(in.Asked) == 0 {
		return Prompt{}, domain.ErrNothingToRead
	}
	var b strings.Builder
	if vocabulary := popularTags(in.Bank); len(vocabulary) > 0 {
		fmt.Fprintf(&b, "Tags already used in the bank (reuse one when it fits): %s\n\n", strings.Join(vocabulary, ", "))
	}
	b.WriteString("Questions to tag (reference, type, level, prompt):\n")
	for _, i := range in.Asked {
		if i < 0 || i >= len(in.Bank) {
			return Prompt{}, fmt.Errorf("prompts: question %d is not in the bank of %d", i, len(in.Bank))
		}
		b.WriteString(cleanupLine(i, in.Bank[i], false))
	}
	b.WriteString("\nFor every question above give 1-4 short lower-case tags and, if it shows level=?, its CEFR level (pre_a1, a1, a2, b1, b2, c1 or c2); level is \"\" if you cannot tell or the question shows one already. " +
		"Mention no question that is not listed. In \"tags\", give each question its reference (such as q3) in \"question\", its tags and its level.")
	return finished(Prompt{Feature: domain.FeatureCleanup, Tier: CleanupTier, System: tagsSystem, User: b.String(), Schema: json.RawMessage(tagsSchema), MaxTokens: cleanupMaxTokens})
}

// DecodeTagging reads the provider's answer to Tagging. A reference that names no question, a question the call did not ask about, a repeated question and a suggestion with no tag are dropped; tags are lower case, at most four, and a level is kept only for a question with none. The suggestions come in bank order.
func DecodeTagging(raw json.RawMessage, in TaggingInput) ([]TagSuggestion, error) {
	var answer struct {
		Tags []rawTag `json:"tags"`
	}
	if err := decode(raw, &answer); err != nil {
		return nil, err
	}
	asked := make(map[int]bool, len(in.Asked))
	for _, i := range in.Asked {
		asked[i] = true
	}
	picked := map[int]TagSuggestion{}
	for _, t := range answer.Tags {
		i, ok := indexOfRef(t.Question, len(in.Bank))
		if _, again := picked[i]; !ok || !asked[i] || again {
			continue
		}
		if tags := lowered(distinct(t.Tags, maxLineTags, maxTagRunes)); len(tags) > 0 {
			picked[i] = TagSuggestion{Question: in.Bank[i], Tags: tags, Level: suggestedLevel(t.Level, in.Bank[i])}
		}
	}
	out := make([]TagSuggestion, 0, len(picked))
	for _, i := range slices.Sorted(maps.Keys(picked)) {
		out = append(out, picked[i])
	}
	return out[:min(len(out), MaxTagSuggestions)], nil
}

// DuplicatesInput is one call that judges candidate pairs: the whole bank, which the references name, and the pairs of its questions whose prompts look alike (domain.SimilarQuestions), best first. Language is the language the reasons are written in.
type DuplicatesInput struct {
	Bank     []domain.BankQuestion
	Pairs    []domain.SimilarPair
	Language domain.Language
}

// Duplicates builds the request that judges which candidate pairs are duplicates. It sends only the questions that are in a pair, each once under its reference in the whole bank, and the pairs, so two questions that no part of the bank holds together are read together here. It answers domain.ErrNothingToRead when there is no pair, an error for a pair outside the bank, and domain.ErrTooLarge for a call longer than MaxCleanupBytes, which FitDuplicates keeps from happening.
func Duplicates(in DuplicatesInput) (Prompt, error) {
	user, err := duplicatesUser(in)
	if err != nil {
		return Prompt{}, err
	}
	return finished(Prompt{Feature: domain.FeatureCleanup, Tier: CleanupTier, System: duplicatesSystem, User: user, Schema: json.RawMessage(duplicatesSchema), MaxTokens: cleanupMaxTokens})
}

// FitDuplicates keeps the most pairs of in, best first, whose Duplicates call is not longer than MaxCleanupBytes, and reports whether it dropped any. The pairs are the pass's own choice and not the teacher's input, so a bank that fills the call with long multi-byte prompts is answered with a smaller call and not refused. It answers an error for a pair outside the bank, as Duplicates does, and keeps no pair for an input that has none.
func FitDuplicates(in DuplicatesInput) (fit DuplicatesInput, dropped bool, err error) {
	if len(in.Pairs) == 0 {
		return in, false, nil
	}
	if _, err := duplicatesUser(in); err != nil {
		return in, false, err
	}
	kept := sort.Search(len(in.Pairs), func(k int) bool {
		user, _ := duplicatesUser(DuplicatesInput{Bank: in.Bank, Pairs: in.Pairs[:k+1], Language: in.Language})
		return len(user) > MaxCleanupBytes
	})
	fit = in
	fit.Pairs = in.Pairs[:kept]
	return fit, kept < len(in.Pairs), nil
}

func duplicatesUser(in DuplicatesInput) (string, error) {
	if len(in.Pairs) == 0 {
		return "", domain.ErrNothingToRead
	}
	var members []int
	for _, p := range in.Pairs {
		if p.A < 0 || p.B <= p.A || p.B >= len(in.Bank) {
			return "", fmt.Errorf("prompts: pair %d, %d is not two questions of the bank of %d", p.A, p.B, len(in.Bank))
		}
		members = append(members, p.A, p.B)
	}
	slices.Sort(members)
	members = slices.Compact(members)
	var b strings.Builder
	b.WriteString("Questions (reference, type, level, tags, prompt):\n")
	for _, i := range members {
		b.WriteString(cleanupLine(i, in.Bank[i], true))
	}
	b.WriteString("\nCandidate pairs (picked because their prompts share many words):\n")
	for _, p := range in.Pairs {
		fmt.Fprintf(&b, "%s %s\n", ref(p.A), ref(p.B))
	}
	fmt.Fprintf(&b, "\nFind: duplicates = at most %d of the candidate pairs whose two questions test the same thing in nearly the same words, the clearest first, each pair once, with a reason.\n"+
		"Write the reasons in %s. Report only pairs from the list above and never a pair that is fine; use [] if no candidate is a duplicate.\n"+
		"Give each pair its two references (such as q1 and q2) in \"a\" and \"b\" and the reason in \"why\".",
		MaxDuplicatesAsked, in.Language.Name())
	return b.String(), nil
}

// DecodeDuplicates reads the provider's answer to Duplicates. A pair is kept only if the call offered it, once, with its questions in bank order and a reason; a reference that names no question, a pair that was not offered and a pair without a reason are dropped, and so is any pair past MaxDuplicatesAsked.
func DecodeDuplicates(raw json.RawMessage, in DuplicatesInput) ([]Duplicate, error) {
	var answer struct {
		Duplicates []rawDuplicate `json:"duplicates"`
	}
	if err := decode(raw, &answer); err != nil {
		return nil, err
	}
	offered := make(map[[2]int]bool, len(in.Pairs))
	for _, p := range in.Pairs {
		offered[[2]int{p.A, p.B}] = true
	}
	out := []Duplicate{}
	kept := map[[2]int]bool{}
	for _, d := range answer.Duplicates {
		if len(out) == MaxDuplicatesAsked {
			break
		}
		a, okA := indexOfRef(d.A, len(in.Bank))
		b, okB := indexOfRef(d.B, len(in.Bank))
		why := clip(oneLine(d.Why), 300)
		pair := [2]int{min(a, b), max(a, b)}
		if !okA || !okB || why == "" || !offered[pair] || kept[pair] {
			continue
		}
		kept[pair] = true
		out = append(out, Duplicate{A: in.Bank[pair[0]], B: in.Bank[pair[1]], Why: why})
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

func finished(p Prompt) (Prompt, error) {
	if len(p.User) > MaxCleanupBytes {
		return Prompt{}, domain.ErrTooLarge
	}
	return p, nil
}

func cleanupLine(i int, q domain.BankQuestion, withTags bool) string {
	level := q.Level
	if level == "" {
		level = "?"
	}
	if !withTags {
		return fmt.Sprintf("%s [%s] level=%s: %s\n", ref(i), q.Type, level, quotedPrompt(q.Prompt))
	}
	tags := "none"
	if shown := lineTags(q.Tags); len(shown) > 0 {
		tags = strings.Join(shown, ",")
	}
	return fmt.Sprintf("%s [%s] level=%s tags=%s: %s\n", ref(i), q.Type, level, tags, quotedPrompt(q.Prompt))
}

func lineTags(tags []string) []string {
	shown := make([]string, 0, maxLineTags)
	for _, tag := range tags {
		if len(shown) == maxLineTags {
			break
		}
		if tag = oneTag(tag); tag != "" {
			shown = append(shown, tag)
		}
	}
	return shown
}

func oneTag(tag string) string {
	return clip(oneLine(strings.NewReplacer(`"`, "'", ",", " ").Replace(tag)), maxTagRunes)
}

func popularTags(bank []domain.BankQuestion) []string {
	counts := map[string]int{}
	for _, q := range bank {
		for _, tag := range lineTags(q.Tags) {
			counts[strings.ToLower(tag)]++
		}
	}
	names := slices.Collect(maps.Keys(counts))
	slices.SortFunc(names, func(a, b string) int { return cmp.Or(cmp.Compare(counts[b], counts[a]), strings.Compare(a, b)) })
	return names[:min(len(names), vocabularySize)]
}

func quotedPrompt(s string) string {
	return `"` + clip(oneLine(strings.ReplaceAll(s, `"`, "'")), 160) + `"`
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
