package prompts

import (
	"encoding/json"
	"fmt"
	"slices"
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
)

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

// Fix is what a finding proposes. OptionID names the option that Text replaces for FixOptionText, and BlankID the blank Text is added to for FixAcceptedAnswer; both are empty for the other kinds.
type Fix struct {
	Kind        string
	Description string
	Label       string
	OptionID    string
	BlankID     string
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

const checkSchema = `{"type":"object","additionalProperties":false,"required":["findings"],"properties":{"findings":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["question","severity","issue","fix"],"properties":{"question":{"type":"integer"},"severity":{"type":"string","enum":["likely_error","worth_a_look","minor"]},"issue":{"type":"string"},"fix":{"type":"object","additionalProperties":false,"required":["kind","description","label","option","blank","text"],"properties":{"kind":{"type":"string","enum":["option_text","accepted_answer","explanation","none"]},"description":{"type":"string"},"label":{"type":"string"},"option":{"type":"string"},"blank":{"type":"integer"},"text":{"type":"string"}}}}}}}}`

var checkWhat = map[string]string{
	CheckKey:      "answer keys that do not match the material or the question, or that two readings could defend",
	CheckClarity:  "unclear or ambiguous wording",
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
		"fix.kind: option_text = replace one option's text (option = its letter, text = the new text); accepted_answer = add an accepted answer to a fill_blank blank (blank = its number, text = the answer); explanation = rewrite the explanation (text); none = advice only. For kinds that do not use them, option is \"\", blank is 0 and text is \"\". label is a button of at most five words.",
		body, strings.Join(what, "; "), in.Language.Name())
	return Prompt{Feature: domain.FeatureCheck, Tier: domain.TierCapable, System: checkSystem, User: user, Schema: json.RawMessage(checkSchema), MaxTokens: 5000}, nil
}

type rawFix struct {
	Kind        string `json:"kind"`
	Description string `json:"description"`
	Label       string `json:"label"`
	Option      string `json:"option"`
	Blank       int    `json:"blank"`
	Text        string `json:"text"`
}

type rawFinding struct {
	Question int    `json:"question"`
	Severity string `json:"severity"`
	Issue    string `json:"issue"`
	Fix      rawFix `json:"fix"`
}

// MaxFindings is the most findings one check returns.
const MaxFindings = 100

// DecodeCheck reads the provider's answer. A finding about a question that is not in the draft, or with no issue, is dropped; a fix that names no option or blank of its question is kept as advice only. The findings come worst first, then in paper order. An empty result is a draft with nothing to report.
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
	text := clip(f.Text, 2000)
	if description == "" {
		advice.Description = clip(issue, 500)
	}
	if text == "" {
		return advice
	}
	fix := Fix{Kind: f.Kind, Description: advice.Description, Label: clip(oneLine(f.Label), 80), Text: text}
	if fix.Label == "" {
		fix.Label = clip(fix.Description, 80)
	}
	switch f.Kind {
	case FixOptionText:
		i := letterIndex(f.Option)
		if q.Rich || i < 0 || i >= len(q.Options) || q.Options[i].ID == "" || sameText(q.Options[i].Text, text) {
			return advice
		}
		fix.OptionID = q.Options[i].ID
	case FixAcceptedAnswer:
		if q.Type != FillBlank || f.Blank < 1 || f.Blank > len(q.Blanks) || q.Blanks[f.Blank-1].ID == "" {
			return advice
		}
		for _, accepted := range q.Blanks[f.Blank-1].Accepted {
			if sameText(accepted, text) {
				return advice
			}
		}
		fix.BlankID = q.Blanks[f.Blank-1].ID
	case FixExplanation:
		if q.Rich {
			return advice
		}
	default:
		return advice
	}
	return fix
}
