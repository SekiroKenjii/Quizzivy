package prompts

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"quizzivy/internal/modules/ai/domain"
)

// The checks a teacher may ask for.
const (
	CheckKey      = "key"
	CheckClarity  = "clarity"
	CheckGiveaway = "giveaway"
	CheckLanguage = "language"
)

// The kinds of one-click fix a finding may carry.
const (
	FixOptionText     = "option_text"
	FixAcceptedAnswer = "accepted_answer"
	FixExplanation    = "explanation"
	FixNone           = "none"
	FixTfngAnswer     = "tfng_answer"
	FixWordLimit      = "word_limit"
)

// MaxWordLimit is the largest word limit a short answer takes.
const MaxWordLimit = 100

// The severities of a finding, worst first.
const (
	LikelyError = "likely_error"
	WorthALook  = "worth_a_look"
	Minor       = "minor"
)

// CheckInput is what "Check this test" sends: the draft's questions with their keys, explanations and shared material, and the checks asked for. Language is the language the findings are written in.
type CheckInput struct {
	Questions []domain.PaperQuestion
	Checks    []string
	Language  domain.Language
}

// Fix is what a finding proposes. OptionID names the option that Text replaces for FixOptionText and the option that becomes a tfng question's key for FixTfngAnswer, where Text is that option's text; BlankID names the blank Text is added to for FixAcceptedAnswer; MaxWords is the word limit FixWordLimit sets, and Text then the rule the student reads. Each is empty where its kind does not use it.
type Fix struct {
	Kind        string
	Description string
	Label       string
	OptionID    string
	BlankID     string
	MaxWords    int
	Text        string
}

// Finding is one thing worth a look in the draft.
type Finding struct {
	Severity   string
	QuestionID string
	Number     int
	Type       string
	Title      string
	Issue      string
	Fix        Fix
}

const checkSystem = "You review English test questions for a teacher and report only real problems. " + untrusted + "Reply with JSON only."

const checkSchema = `{"type":"object","additionalProperties":false,"required":["findings"],"properties":{"findings":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["question","severity","issue","fix"],"properties":{"question":{"type":"integer"},"severity":{"type":"string","enum":["likely_error","worth_a_look","minor"]},"issue":{"type":"string"},"fix":{"type":"object","additionalProperties":false,"required":["kind","description","label","option","blank","words","text"],"properties":{"kind":{"type":"string","enum":["option_text","accepted_answer","explanation","tfng_answer","word_limit","none"]},"description":{"type":"string"},"label":{"type":"string"},"option":{"type":"string"},"blank":{"type":"integer"},"words":{"type":"integer"},"text":{"type":"string"}}}}}}}}`

var checkWhat = map[string]string{
	CheckKey:      "answer keys that do not match the material or the question, or that two readings could defend, such as a tfng key of False where the material never says, which is Not given, a matching item keyed to an option that does not fit it, or a \"Choose exactly N\" question with other than N correct options",
	CheckClarity:  "unclear or ambiguous wording, and a short_answer with no word limit, whose answers will vary in length",
	CheckGiveaway: "questions that give their answer away, such as one option repeating a word from the passage that the others lack",
	CheckLanguage: "spelling and grammar mistakes",
}

// Check builds the request for "Check this test". It answers domain.ErrNothingToRead for a draft with no question and domain.ErrTooLarge for one longer than MaxReadChars.
func Check(in CheckInput) (Prompt, error) {
	if len(in.Questions) == 0 {
		return Prompt{}, domain.ErrNothingToRead
	}
	body, err := paperText(in.Questions)
	if err != nil {
		return Prompt{}, err
	}
	var what []string
	for _, c := range in.Checks {
		what = append(what, checkWhat[c])
	}
	user := fmt.Sprintf("%s\nLook for: %s.\nReport only real problems; a question with none is not mentioned.\n"+
		"question = the question's number. severity: likely_error = an answer that is wrong, worth_a_look = doubtful or ambiguous, minor = polish.\n"+
		"Write issue, description and label in %s; text keeps the language of the question.\n"+
		"fix.kind: option_text = replace one option's text (option = its letter, text = the new text); accepted_answer = add an accepted answer to a fill_blank blank (blank = its number, text = the answer); explanation = rewrite the explanation (text); tfng_answer = make another option of a tfng question its answer (option = the letter of the new answer, such as C for Not given); word_limit = give a short_answer with no word limit one (words = the most words an answer may have, 1-%d); none = advice only. For kinds that do not use them, option is \"\", blank is 0, words is 0 and text is \"\". label is a button of at most five words, such as \"Change to Not given\" or \"Add word limit\".",
		body, strings.Join(what, "; "), in.Language.Name(), MaxWordLimit)
	return Prompt{Feature: domain.FeatureCheck, Tier: domain.TierCapable, System: checkSystem, User: user, Schema: json.RawMessage(checkSchema), MaxTokens: 5000}, nil
}

type rawFix struct {
	Kind        string          `json:"kind"`
	Description string          `json:"description"`
	Label       string          `json:"label"`
	Option      string          `json:"option"`
	Blank       int             `json:"blank"`
	Words       json.RawMessage `json:"words"`
	Text        string          `json:"text"`
}

type rawFinding struct {
	Question int    `json:"question"`
	Severity string `json:"severity"`
	Issue    string `json:"issue"`
	Fix      rawFix `json:"fix"`
}

// MaxFindings is the most findings one check returns.
const MaxFindings = 100

// DecodeCheck reads the provider's answer. A finding about a question that is not in the draft, or with no issue, is dropped; a fix that names no option or blank of its question, a tfng_answer that names no other option of a tfng question, and a word_limit on anything but a short answer with no limit, or with a words count that is not a whole number from 1 to MaxWordLimit, are kept as advice only. The findings come worst first, then in paper order. An empty result is a draft with nothing to report.
func DecodeCheck(raw json.RawMessage, in CheckInput) ([]Finding, error) {
	var answer struct {
		Findings []rawFinding `json:"findings"`
	}
	if err := decode(raw, &answer); err != nil {
		return nil, err
	}
	out := []Finding{}
	for _, f := range answer.Findings {
		q, ok := questionByNumber(in.Questions, f.Question)
		issue := clip(f.Issue, 1000)
		if !ok || issue == "" || !slices.Contains([]string{LikelyError, WorthALook, Minor}, f.Severity) {
			continue
		}
		out = append(out, Finding{
			Severity: f.Severity, QuestionID: q.ID, Number: q.Number, Type: q.Type,
			Title: clip(oneLine(q.Prompt), 140), Issue: issue, Fix: tidyFix(f.Fix, issue, q),
		})
	}
	rank := map[string]int{LikelyError: 0, WorthALook: 1, Minor: 2}
	slices.SortStableFunc(out, func(a, b Finding) int {
		if rank[a.Severity] != rank[b.Severity] {
			return rank[a.Severity] - rank[b.Severity]
		}
		return a.Number - b.Number
	})
	if len(out) > MaxFindings {
		out = out[:MaxFindings]
	}
	return out, nil
}

func tidyFix(f rawFix, issue string, q domain.PaperQuestion) Fix {
	description := clip(f.Description, 500)
	advice := Fix{Kind: FixNone, Description: description}
	if description == "" {
		advice.Description = clip(issue, 500)
	}
	fix := Fix{Kind: f.Kind, Description: advice.Description, Label: clip(oneLine(f.Label), 80), Text: clip(f.Text, 2000)}
	if fix.Label == "" {
		fix.Label = clip(fix.Description, 80)
	}
	ok := false
	switch f.Kind {
	case FixOptionText:
		fix.OptionID, ok = replaceableOption(f.Option, fix.Text, q)
	case FixAcceptedAnswer:
		fix.BlankID, ok = extendableBlank(f.Blank, fix.Text, q)
	case FixExplanation:
		ok = fix.Text != "" && !q.Rich
	case FixTfngAnswer:
		fix.OptionID, fix.Text, ok = tfngKey(f.Option, q)
	case FixWordLimit:
		words, whole := wholeNumber(f.Words)
		ok = whole && q.Type == ShortAnswer && q.MaxWords == 0 && words >= 1 && words <= MaxWordLimit
		fix.MaxWords, fix.Text = words, WordLimitRule(words)
	}
	if !ok {
		return advice
	}
	return fix
}

// wholeNumber reads a JSON number with no fraction, such as 3 or 3.0; anything else, a string or a bool included, is false, so one odd field leaves its fix advice rather than failing the check.
func wholeNumber(raw json.RawMessage) (int, bool) {
	var v float64
	if json.Unmarshal(raw, &v) != nil || v != math.Trunc(v) || math.Abs(v) > 1e6 {
		return 0, false
	}
	return int(v), true
}

func tfngKey(letter string, q domain.PaperQuestion) (string, string, bool) {
	i, labels := letterIndex(letter), TfngOptions()
	if q.Type != Tfng || len(q.Options) != len(labels) || i < 0 || i >= len(labels) || q.Options[i].ID == "" || q.Options[i].Correct {
		return "", "", false
	}
	return q.Options[i].ID, labels[i], true
}

var numberWords = []string{"", "ONE", "TWO", "THREE", "FOUR", "FIVE", "SIX", "SEVEN", "EIGHT", "NINE", "TEN", "ELEVEN"}

// WordLimitRule is the rule a word limit prints under a short answer, as the engine writes it in English: "Write ONE WORD." for one, and "Write NO MORE THAN THREE WORDS." with the number in words up to eleven and in digits above.
func WordLimitRule(limit int) string {
	if limit == 1 {
		return "Write ONE WORD."
	}
	words := strconv.Itoa(limit)
	if limit > 1 && limit < len(numberWords) {
		words = numberWords[limit]
	}
	return "Write NO MORE THAN " + words + " WORDS."
}

func replaceableOption(letter, text string, q domain.PaperQuestion) (string, bool) {
	i := letterIndex(letter)
	if q.Rich || q.Type == Tfng || text == "" || i < 0 || i >= len(q.Options) || q.Options[i].ID == "" || sameText(q.Options[i].Text, text) {
		return "", false
	}
	return q.Options[i].ID, true
}

func extendableBlank(number int, text string, q domain.PaperQuestion) (string, bool) {
	if text == "" || q.Type != FillBlank || number < 1 || number > len(q.Blanks) || q.Blanks[number-1].ID == "" {
		return "", false
	}
	blank := q.Blanks[number-1]
	if slices.ContainsFunc(blank.Accepted, func(accepted string) bool { return sameText(accepted, text) }) {
		return "", false
	}
	return blank.ID, true
}
