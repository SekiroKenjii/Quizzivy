package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	questions "quizzivy/internal/modules/questions/domain"
	tests "quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/content"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

// ErrNotReady rejects a commit while blocking findings or undecided review items remain.
var ErrNotReady = errors.New("imports: draft not ready")

const maxTitle = 200

// CommitPlan is one draft test described with fresh persistent IDs, so importing the same file twice never collides.
type CommitPlan struct {
	Title    string
	Sections []PlanSection
}

type PlanSection struct {
	Title        string
	Instructions *string
	Units        []PlanUnit
}

// PlanUnit holds exactly one of a standalone bank question or a shared-passage group.
type PlanUnit struct {
	Question *questions.Input
	Group    *tests.GroupBundle
}

// Input maps an included draft question onto the bank's write model.
func (q DraftQuestion) Input() (questions.Input, error) {
	in := questions.Input{Type: questions.Type(q.Type), PromptContent: q.Prompt, Prompt: plainText(q.Prompt), Points: q.Points, Tags: []string{}}
	if in.Prompt == "" {
		return in, content.ErrInvalidDocument
	}
	for _, o := range q.Options {
		in.Options = append(in.Options, questions.OptionInput{Content: o.Content, Text: plainText(o.Content), IsCorrect: slices.Contains(q.Answer.OptionIDs, o.ID)})
	}
	for i, b := range q.Blanks {
		gap := b.GapID
		in.Blanks = append(in.Blanks, questions.BlankInput{GapID: &gap, Ordinal: i + 1, AcceptedAnswers: slices.Clone(b.Accepted), CaseSensitive: b.CaseSensitive})
	}
	if q.Type == string(questions.ShortAnswer) && strings.TrimSpace(q.Answer.Text) != "" {
		sample := q.Answer.Text
		in.SampleAnswer = &sample
	}
	return in, nil
}

// Plan converts a ready draft into the inputs of one draft test; excluded questions and empty sections are left out.
func Plan(d Draft, fallbackTitle string, newID func() string) (CommitPlan, error) {
	if !Assess(d).Ready {
		return CommitPlan{}, ErrNotReady
	}
	plan := CommitPlan{Title: clip(firstNonEmpty(d.Title, fallbackTitle))}
	for _, s := range d.Sections {
		section := PlanSection{Title: clip(s.Title)}
		if text := strings.TrimSpace(s.Instructions); text != "" {
			section.Instructions = &text
		}
		for _, it := range s.Items {
			unit, ok, err := planUnit(it, s.Title, newID)
			if err != nil {
				return CommitPlan{}, err
			}
			if ok {
				section.Units = append(section.Units, unit)
			}
		}
		if len(section.Units) > 0 {
			plan.Sections = append(plan.Sections, section)
		}
	}
	return plan, nil
}

func planUnit(it DraftItem, sectionTitle string, newID func() string) (PlanUnit, bool, error) {
	if it.Question != nil {
		if it.Question.Excluded != nil {
			return PlanUnit{}, false, nil
		}
		in, err := it.Question.Input()
		return PlanUnit{Question: &in}, err == nil, err
	}
	bundle, err := groupBundle(*it.Group, sectionTitle, newID)
	if err != nil || len(bundle.Questions) == 0 {
		return PlanUnit{}, false, err
	}
	return PlanUnit{Group: &bundle}, true, nil
}

func groupBundle(g DraftGroup, sectionTitle string, newID func() string) (tests.GroupBundle, error) {
	ids := map[string]string{}
	bundle := tests.GroupBundle{Group: tests.QuestionGroup{ID: newID(), Title: clip(firstNonEmpty(g.Instructions, sectionTitle, "Group "+g.Label)), Members: []tests.GroupMember{}, Stimuli: []tests.GroupStimulus{}, Recordings: []tests.GroupRecording{}}}
	for _, q := range g.Questions {
		if q.Excluded != nil {
			continue
		}
		in, err := q.Input()
		if err != nil {
			return tests.GroupBundle{}, err
		}
		ids[q.ID] = newID()
		order := "shuffle"
		if isChoice(q.Type) {
			order = "fixed"
		}
		bundle.Questions = append(bundle.Questions, tests.GroupQuestion{ID: ids[q.ID], Input: in})
		bundle.Group.Members = append(bundle.Group.Members, tests.GroupMember{QuestionID: ids[q.ID], OptionOrder: order})
	}
	if len(g.Stimulus) == 0 || len(bundle.Questions) == 0 {
		return bundle, nil
	}
	stimulus := tests.GroupStimulus{ID: newID(), Title: bundle.Group.Title, Gaps: []tests.GroupGapBinding{}}
	kept := map[string]bool{}
	for _, link := range g.Gaps {
		target, ok := ids[link.QuestionID]
		if !ok {
			continue
		}
		binding := tests.GroupGapBinding{Kind: "question", GapID: link.GapID, QuestionID: target}
		if link.BlankGapID != "" {
			blank := link.BlankGapID
			binding.Kind, binding.BlankGapID = "blank", &blank
		}
		stimulus.Gaps = append(stimulus.Gaps, binding)
		kept[link.GapID] = true
	}
	raw, err := unbindGaps(g.Stimulus, kept)
	if err != nil {
		return tests.GroupBundle{}, err
	}
	stimulus.Content = raw
	bundle.Group.Stimuli = append(bundle.Group.Stimuli, stimulus)
	return bundle, nil
}

func validGroupShape(g DraftGroup) error {
	n := 0
	bundle, err := groupBundle(g, "Group", func() string {
		n++
		return uuid.NewSHA1(uuid.NameSpaceOID, []byte(g.ID+"/"+strconv.Itoa(n))).String()
	})
	if err != nil {
		return err
	}
	return bundle.Validate()
}

func unbindGaps(raw json.RawMessage, kept map[string]bool) (json.RawMessage, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(replaceGaps(value, kept)); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(out.Bytes()), nil
}

func replaceGaps(value any, kept map[string]bool) any {
	switch node := value.(type) {
	case map[string]any:
		if node["type"] == "gap" {
			id, _ := node["id"].(string)
			if kept[id] {
				return node
			}
			label, _ := node["label"].(string)
			return map[string]any{"type": "text", "text": "(" + label + ")", "marks": []any{}}
		}
		for key, child := range node {
			node[key] = replaceGaps(child, kept)
		}
		return node
	case []any:
		for i := range node {
			node[i] = replaceGaps(node[i], kept)
		}
		return node
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func clip(text string) string {
	if utf8.RuneCountInString(text) <= maxTitle {
		return text
	}
	return strings.TrimSpace(string([]rune(text)[:maxTitle-1])) + "…"
}
