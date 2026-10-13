package prompts

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"quizzivy/internal/modules/ai/domain"
)

// The changes a variant may make.
const (
	VariantReword   = "reword"
	VariantReorder  = "reorder"
	VariantDistract = "distract"
)

// The difficulties a variant may aim for.
const (
	DifficultySame   = "same"
	DifficultyEasier = "easier"
	DifficultyHarder = "harder"
)

// VariantInput is what "Make a variant" sends: the draft's questions with their options and keys, what to change and how hard to aim.
type VariantInput struct {
	Questions  []domain.PaperQuestion
	Changes    []string
	Difficulty string
}

// Change is one proposed change to a question. Before and After are for the eye. The rest is what applying it writes: Prompt for VariantReword, OptionOrder (every option's id, in the new order) for VariantReorder, OptionID and Text for VariantDistract.
type Change struct {
	QuestionID  string
	Number      int
	Kind        string
	Title       string
	Before      string
	After       string
	Prompt      string
	OptionOrder []string
	OptionID    string
	Text        string
}

const variantSystem = "You make a variant of an English test so students cannot share answers. The answers, the points and the passages never change. " + untrusted + "Reply with JSON only."

const variantSchema = `{"type":"object","additionalProperties":false,"required":["changes"],"properties":{"changes":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["question","kind","prompt","order","option","text"],"properties":{"question":{"type":"integer"},"kind":{"type":"string","enum":["reword","reorder","distract"]},"prompt":{"type":"string"},"order":{"type":"array","items":{"type":"string"}},"option":{"type":"string"},"text":{"type":"string"}}}}}}`

var variantWhat = map[string]string{
	VariantReword:   "reword = write a question's prompt differently with the same meaning and the same answer",
	VariantReorder:  "reorder = put a choice question's options in another order (order = the option letters in the new order)",
	VariantDistract: "distract = replace one wrong option with a new wrong option (option = the letter of the wrong option, text = the new option)",
}

var variantAim = map[string]string{
	DifficultySame:   "Keep the difficulty the same.",
	DifficultyEasier: "Make the wording a little easier.",
	DifficultyHarder: "Make the wording a little harder.",
}

// Variant builds the request for "Make a variant". It answers domain.ErrNothingToRead for a draft with no question and domain.ErrTooLarge for one longer than MaxReadChars.
func Variant(in VariantInput) (Prompt, error) {
	if len(in.Questions) == 0 {
		return Prompt{}, domain.ErrNothingToRead
	}
	body, err := paperText(in.Questions)
	if err != nil {
		return Prompt{}, err
	}
	var what []string
	for _, c := range in.Changes {
		what = append(what, variantWhat[c])
	}
	user := fmt.Sprintf("%s\nMake these changes: %s.\n%s Only the wording changes, never the answer.\n"+
		"Never make a wrong option correct. Keep every ___ gap and the language of each question. Make at most one change of each kind to a question, and skip a question you cannot vary safely.\n"+
		"For kinds that do not use them, prompt is \"\", order is [], option is \"\" and text is \"\".",
		body, strings.Join(what, "; "), variantAim[in.Difficulty])
	return Prompt{Feature: domain.FeatureVariant, Tier: domain.TierCapable, System: variantSystem, User: user, Schema: json.RawMessage(variantSchema), MaxTokens: 6000}, nil
}

type rawChange struct {
	Question int      `json:"question"`
	Kind     string   `json:"kind"`
	Prompt   string   `json:"prompt"`
	Order    []string `json:"order"`
	Option   string   `json:"option"`
	Text     string   `json:"text"`
}

// MaxChanges is the most changes one variant proposes.
const MaxChanges = 300

// DecodeVariant reads the provider's answer. It keeps a change only when it can be applied as proposed: the question exists, the kind was asked for and fits the question, a reorder is a different permutation of its options, a new wrong option is new and replaces a wrong one, and a reword is a different prompt of a question whose text is plain. If none is left the answer is ErrUnusable.
func DecodeVariant(raw json.RawMessage, in VariantInput) ([]Change, error) {
	var answer struct {
		Changes []rawChange `json:"changes"`
	}
	if err := decode(raw, &answer); err != nil {
		return nil, err
	}
	type key struct {
		number int
		kind   string
	}
	seen := map[key]bool{}
	var out []Change
	for _, c := range answer.Changes {
		q, ok := questionByNumber(in.Questions, c.Question)
		if !ok || !slices.Contains(in.Changes, c.Kind) || seen[key{c.Question, c.Kind}] {
			continue
		}
		change, ok := tidyChange(c, q)
		if !ok {
			continue
		}
		seen[key{c.Question, c.Kind}] = true
		out = append(out, change)
	}
	if len(out) == 0 {
		return nil, ErrUnusable
	}
	slices.SortStableFunc(out, func(a, b Change) int { return a.Number - b.Number })
	if len(out) > MaxChanges {
		out = out[:MaxChanges]
	}
	return out, nil
}

func choiceQuestion(q domain.PaperQuestion) bool {
	return q.Type == SingleChoice || q.Type == MultipleChoice
}

func tidyChange(c rawChange, q domain.PaperQuestion) (Change, bool) {
	out := Change{QuestionID: q.ID, Number: q.Number, Kind: c.Kind, Title: clip(oneLine(q.Prompt), 140)}
	switch c.Kind {
	case VariantReword:
		prompt := strings.TrimSpace(c.Prompt)
		if q.Rich || q.Type == FillBlank || prompt == "" || len([]rune(prompt)) > 2000 || sameText(prompt, q.Prompt) {
			return Change{}, false
		}
		out.Before, out.After, out.Prompt = clip(q.Prompt, 2000), prompt, prompt
	case VariantReorder:
		order, ok := permutation(c.Order, len(q.Options))
		if !choiceQuestion(q) || q.OrderFixed || len(q.Options) < 3 || !ok {
			return Change{}, false
		}
		out.Before, out.After = describeOrder(q.Options, identity(len(q.Options))), describeOrder(q.Options, order)
		for _, i := range order {
			if q.Options[i].ID == "" {
				return Change{}, false
			}
			out.OptionOrder = append(out.OptionOrder, q.Options[i].ID)
		}
	case VariantDistract:
		i, text := letterIndex(c.Option), clip(oneLine(c.Text), 500)
		if !choiceQuestion(q) || q.Rich || i < 0 || i >= len(q.Options) || q.Options[i].Correct || q.Options[i].ID == "" || text == "" {
			return Change{}, false
		}
		for _, o := range q.Options {
			if sameText(o.Text, text) {
				return Change{}, false
			}
		}
		out.Before, out.After = letter(i)+" · "+clip(q.Options[i].Text, 500), letter(i)+" · "+text
		out.OptionID, out.Text = q.Options[i].ID, text
	default:
		return Change{}, false
	}
	return out, true
}

func identity(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

func permutation(letters []string, n int) ([]int, bool) {
	if len(letters) != n {
		return nil, false
	}
	order := make([]int, n)
	used := make([]bool, n)
	for k, l := range letters {
		i := letterIndex(l)
		if i < 0 || i >= n || used[i] {
			return nil, false
		}
		used[i], order[k] = true, i
	}
	return order, !slices.Equal(order, identity(n))
}

func describeOrder(options []domain.Option, order []int) string {
	parts := make([]string, len(order))
	for k, i := range order {
		parts[k] = letter(i)
		if options[i].Correct {
			parts[k] += " (correct)"
		}
	}
	return strings.Join(parts, " · ")
}
