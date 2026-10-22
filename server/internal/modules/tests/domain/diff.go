package domain

import (
	"fmt"
	questionsdomain "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/shared/matchkey"
	"strings"
	"unicode/utf8"
)

// ChangeKind names what a Change reports.
type ChangeKind string

const (
	ChangeAdded   ChangeKind = "added"
	ChangeRemoved ChangeKind = "removed"
	ChangeChanged ChangeKind = "changed"
	ChangeAnswer  ChangeKind = "answer"
	ChangePoints  ChangeKind = "points"
)

// ChangedField is a part of a question that a ChangeChanged entry names.
type ChangedField string

const (
	FieldType        ChangedField = "type"
	FieldPrompt      ChangedField = "prompt"
	FieldOptions     ChangedField = "options"
	FieldBlanks      ChangedField = "blanks"
	FieldItems       ChangedField = "items"
	FieldRubric      ChangedField = "rubric"
	FieldRules       ChangedField = "rules"
	FieldMedia       ChangedField = "media"
	FieldPoints      ChangedField = "points"
	FieldExplanation ChangedField = "explanation"
	FieldContext     ChangedField = "context"
	FieldSection     ChangedField = "section"
)

// MaxChangePrompt is the longest prompt a Change carries, in characters.
const MaxChangePrompt = 200

// Change is one difference between two papers. Number and QuestionID name the
// question in the paper the number counts in: the second paper for every kind
// but ChangeRemoved, which counts in the first. ChangePoints names no question.
type Change struct {
	Kind       ChangeKind
	Number     int
	QuestionID string
	// Prompt is the question's plain-text prompt, cut to MaxChangePrompt
	// characters, for ChangeAdded and ChangeRemoved.
	Prompt string
	// Fields is what differs, for ChangeChanged.
	Fields []ChangedField
	// AnswerFrom and AnswerTo are the labels of the correct options of a
	// choice or TFNG question, or of each matching item and its option, for
	// ChangeAnswer.
	AnswerFrom []string
	AnswerTo   []string
	// PointsFrom and PointsTo are the two totals, for ChangePoints.
	PointsFrom string
	PointsTo   string
}

// Compare lists what differs between two papers: a question in only one of
// them is added or removed, one in both is changed when a part of it differs
// and answer when its key does, and the totals differ at most once as points.
// The changes are ordered by kind, in the order of the Change constants, and
// then by question number.
//
// Questions are matched by SourceID, the bank question they come from, and the
// questions left over by identical content, because restoring a version as a
// draft copies its questions into new bank rows. When several questions of one
// paper share a key, they pair with those of the other in paper order. Moving
// a question, so that the numbers shift, is not a change. It fails when a
// question holds points that are not a score, which stored data does not.
func Compare(from, to DraftContent) ([]Change, error) {
	e := &encoder{}
	return compare(e, newPaper(e, from), newPaper(e, to), true)
}

// Introduction lists every question of a paper as added, with no points entry:
// what a first version holds against nothing. It fails as Compare does.
func Introduction(to DraftContent) ([]Change, error) {
	e := &encoder{}
	return compare(e, paper{}, newPaper(e, to), false)
}

func compare(e *encoder, from, to paper, withTotal bool) ([]Change, error) {
	if e.err != nil {
		return nil, e.err
	}
	pairs := match(from, to)
	fromLabels, toLabels := pairLabels(from, to, pairs)
	fromContexts := newContexts(e, from, fromLabels)
	toContexts := newContexts(e, to, toLabels)

	var added, removed, changed, answers []Change
	for j, q := range to.questions {
		i := pairs.toFrom[j]
		if i < 0 {
			added = append(added, Change{Kind: ChangeAdded, Number: q.number, QuestionID: q.id, Prompt: shortPrompt(q.question.Prompt)})
			continue
		}
		p := from.questions[i]
		fields := changedFields(p, q, fromContexts.of(p), toContexts.of(q))
		if len(fields) > 0 {
			changed = append(changed, Change{Kind: ChangeChanged, Number: q.number, QuestionID: q.id, Fields: fields})
		}
		if p.parts.answer != q.parts.answer {
			answers = append(answers, answerChange(p, q))
		}
	}
	for i, p := range from.questions {
		if pairs.fromTo[i] < 0 {
			removed = append(removed, Change{Kind: ChangeRemoved, Number: p.number, QuestionID: p.id, Prompt: shortPrompt(p.question.Prompt)})
		}
	}

	out := append(added, removed...)
	out = append(out, changed...)
	out = append(out, answers...)
	if withTotal && from.total != to.total {
		out = append(out, Change{Kind: ChangePoints, PointsFrom: formatPoints(from.total), PointsTo: formatPoints(to.total)})
	}
	if e.err != nil {
		return nil, e.err
	}
	return out, nil
}

func changedFields(from, to paperQuestion, fromContext, toContext string) []ChangedField {
	var fields []ChangedField
	for _, c := range []struct {
		field ChangedField
		same  bool
	}{
		{FieldType, from.parts.kind == to.parts.kind},
		{FieldPrompt, from.parts.prompt == to.parts.prompt},
		{FieldOptions, from.parts.options == to.parts.options},
		{FieldBlanks, from.parts.blanks == to.parts.blanks},
		{FieldItems, from.parts.items == to.parts.items},
		{FieldRubric, from.parts.rubric == to.parts.rubric},
		{FieldRules, from.parts.rules == to.parts.rules},
		{FieldMedia, from.parts.media == to.parts.media},
		{FieldPoints, from.parts.points == to.parts.points},
		{FieldExplanation, from.parts.explanation == to.parts.explanation},
		{FieldContext, fromContext == toContext},
		{FieldSection, from.section == to.section},
	} {
		if !c.same {
			fields = append(fields, c.field)
		}
	}
	return fields
}

func answerChange(from, to paperQuestion) Change {
	change := Change{Kind: ChangeAnswer, Number: to.number, QuestionID: to.id}
	fromKind, toKind := questionsdomain.Type(from.parts.kind), questionsdomain.Type(to.parts.kind)
	switch {
	case fromKind == questionsdomain.Matching && toKind == questionsdomain.Matching:
		change.AnswerFrom = matchLabels(from.parts.keys)
		change.AnswerTo = matchLabels(to.parts.keys)
	case keyedByOption(fromKind) && keyedByOption(toKind):
		change.AnswerFrom = optionLabels(fromKind, from.parts.correct)
		change.AnswerTo = optionLabels(toKind, to.parts.correct)
	}
	return change
}

// tfngValues name a tfng question's options, in their fixed order, in a
// diff: the reader shows each value in its own language, not a letter.
var tfngValues = [3]string{"true", "false", "not_given"}

func keyedByOption(t questionsdomain.Type) bool {
	return t.IsChoice() || t == questionsdomain.TFNG
}

// matchLabels names each item by its key and the key of the choice it is
// matched to, as the editor draws them (D-063): "A → iii", or "A → –" when
// the item is keyed to none.
func matchLabels(keys []int) []string {
	labels := make([]string, len(keys))
	for i, key := range keys {
		option := "–"
		if key >= 0 {
			option = matchkey.Option(key)
		}
		labels[i] = matchkey.Item(i) + " → " + option
	}
	return labels
}

func optionLabels(kind questionsdomain.Type, positions []int) []string {
	labels := make([]string, len(positions))
	for i, position := range positions {
		if kind == questionsdomain.TFNG && position < len(tfngValues) {
			labels[i] = tfngValues[position]
		} else {
			labels[i] = optionLabel(position)
		}
	}
	return labels
}

func optionLabel(position int) string {
	if position < 26 {
		return string(rune('A' + position))
	}
	return fmt.Sprint(position + 1)
}

func shortPrompt(prompt string) string {
	prompt = strings.TrimSpace(prompt)
	if utf8.RuneCountInString(prompt) <= MaxChangePrompt {
		return prompt
	}
	runes := []rune(prompt)
	return strings.TrimSpace(string(runes[:MaxChangePrompt-1])) + "…"
}

func formatPoints(units int64) string {
	return fmt.Sprintf("%d.%02d", units/100, units%100)
}
