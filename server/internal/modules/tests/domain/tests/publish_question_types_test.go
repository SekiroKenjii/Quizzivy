package domain_test

import (
	"slices"
	"testing"

	"quizzivy/internal/modules/tests/domain"
)

func intOf(n int) *int { return &n }

func boolOf(b bool) *bool { return &b }

func options(correct ...bool) []domain.DraftOption {
	out := make([]domain.DraftOption, len(correct))
	for i, isCorrect := range correct {
		out[i] = domain.DraftOption{Ordinal: i, Text: string(rune('a' + i)), IsCorrect: isCorrect}
	}
	return out
}

func tfngQuestion(correct ...bool) domain.DraftQuestion {
	q := base()
	q.Type, q.Options = "tfng", options(correct...)
	return q
}

func chooseTwo(correct ...bool) domain.DraftQuestion {
	q := base()
	q.Type, q.SelectCount, q.Options = "multiple_choice", intOf(2), options(correct...)
	return q
}

func matchingQuestion(keys ...int) domain.DraftQuestion {
	q := base()
	q.Type, q.MatchAllowReuse, q.Options = "matching", boolOf(false), options(false, false, false)
	for i, key := range keys {
		q.MatchItems = append(q.MatchItems, domain.DraftMatchItem{Ordinal: i, Text: "item " + string(rune('A'+i)), CorrectOption: key})
	}
	return q
}

func essayQuestion(labels ...string) domain.DraftQuestion {
	q := base()
	q.Type = "essay"
	for i, label := range labels {
		q.Rubric = append(q.Rubric, domain.DraftCriterion{Ordinal: i, Label: label})
	}
	return q
}

func TestEveryNewTypePublishesWhenItIsWhole(t *testing.T) {
	short := base()
	short.MaxWords = intOf(40)
	for name, q := range map[string]domain.DraftQuestion{
		"tfng":           tfngQuestion(false, false, true),
		"choose two":     chooseTwo(true, false, true, false),
		"word limit":     short,
		"matching":       matchingQuestion(2, 0, 1),
		"essay":          essayQuestion("Task response", "Coherence"),
		"essay, no rubr": essayQuestion(),
	} {
		if err := domain.Publishing.Validate(draftWith(q)); err != nil {
			t.Errorf("%s: %v, want it to publish", name, err)
		}
	}
}

func TestAChooseNWhoseCorrectCountDiffersFromNIsRefusedAsAMissingKey(t *testing.T) {
	for _, correct := range [][]bool{{true, false, false, false}, {true, true, true, false}} {
		v := onlyViolation(t, draftWith(chooseTwo(correct...)))
		if v.Rule != domain.ChoiceHasCorrectOption {
			t.Errorf("%v correct gave rule %s, want %s", correct, v.Rule, domain.ChoiceHasCorrectOption)
		}
	}
}

func TestATFNGWithoutExactlyOneCorrectOptionIsRefused(t *testing.T) {
	for _, correct := range [][]bool{{false, false, false}, {true, false, true}} {
		v := onlyViolation(t, draftWith(tfngQuestion(correct...)))
		if v.Rule != domain.ChoiceHasCorrectOption {
			t.Errorf("%v correct gave rule %s, want %s", correct, v.Rule, domain.ChoiceHasCorrectOption)
		}
	}
}

func TestAMatchingItemKeyedToNoChoiceIsRefused(t *testing.T) {
	for _, keys := range [][]int{{0, -1, 1}, {0, 3, 1}} {
		v := onlyViolation(t, draftWith(matchingQuestion(keys...)))
		if v.Rule != domain.ChoiceHasCorrectOption {
			t.Errorf("keys %v gave rule %s, want %s", keys, v.Rule, domain.ChoiceHasCorrectOption)
		}
		if v.QuestionID != base().SourceID {
			t.Errorf("keys %v: violation names %q", keys, v.QuestionID)
		}
	}
}

func TestAFieldOfAnotherTypeIsRefusedAtPublish(t *testing.T) {
	q := base()
	q.SelectCount = intOf(2)
	if v := onlyViolation(t, draftWith(q)); v.Rule != domain.QuestionValid {
		t.Errorf("a short answer with selectCount gave rule %s, want %s", v.Rule, domain.QuestionValid)
	}
	q = essayQuestion("Task", "task")
	if v := onlyViolation(t, draftWith(q)); v.Rule != domain.QuestionValid {
		t.Errorf("an essay with two equal criteria gave rule %s, want %s", v.Rule, domain.QuestionValid)
	}
}

func onlyChange(t *testing.T, from, to domain.DraftQuestion) []domain.Change {
	t.Helper()
	from.SourceID, to.SourceID = "q", "q"
	return compare(t, onePart(from), onePart(to))
}

func wantFields(t *testing.T, name string, changes []domain.Change, want ...domain.ChangedField) {
	t.Helper()
	var got []domain.ChangedField
	for _, c := range changes {
		if c.Kind == domain.ChangeChanged {
			got = append(got, c.Fields...)
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf("%s: changed fields %v, want %v", name, got, want)
	}
}

func hasAnswer(changes []domain.Change) bool {
	return slices.ContainsFunc(changes, func(c domain.Change) bool { return c.Kind == domain.ChangeAnswer })
}

func TestTheDiffNamesEachNewFieldAsAChange(t *testing.T) {
	short := base()
	short.MaxWords = intOf(40)
	shorter := short
	shorter.MaxWords = intOf(20)

	renamedItem := matchingQuestion(0, 1, 2)
	renamedItem.MatchItems = slices.Clone(renamedItem.MatchItems)
	renamedItem.MatchItems[1].Text = "another item"

	renamedChoice := matchingQuestion(0, 1, 2)
	renamedChoice.Options = slices.Clone(renamedChoice.Options)
	renamedChoice.Options[2].Text = "another choice"

	reuse := matchingQuestion(0, 1, 2)
	reuse.MatchAllowReuse = boolOf(true)

	chooseThree := chooseTwo(true, true, true, false)
	chooseThree.SelectCount = intOf(3)

	for _, c := range []struct {
		name     string
		from, to domain.DraftQuestion
		want     []domain.ChangedField
		answer   bool
	}{
		{"maxWords", short, shorter, []domain.ChangedField{domain.FieldRules}, false},
		{"selectCount", chooseTwo(true, true, false, false), chooseThree, []domain.ChangedField{domain.FieldRules}, true},
		{"matchAllowReuse", matchingQuestion(0, 1, 2), reuse, []domain.ChangedField{domain.FieldRules}, false},
		{"match item text", matchingQuestion(0, 1, 2), renamedItem, []domain.ChangedField{domain.FieldItems}, false},
		{"choice text", matchingQuestion(0, 1, 2), renamedChoice, []domain.ChangedField{domain.FieldOptions}, false},
		{"rubric label", essayQuestion("Task", "Lexis"), essayQuestion("Task", "Vocabulary"), []domain.ChangedField{domain.FieldRubric}, false},
		{"rubric added", essayQuestion("Task"), essayQuestion("Task", "Grammar"), []domain.ChangedField{domain.FieldRubric}, false},
	} {
		changes := onlyChange(t, c.from, c.to)
		wantFields(t, c.name, changes, c.want...)
		if hasAnswer(changes) != c.answer {
			t.Errorf("%s: answer entry %v, want %v (%+v)", c.name, hasAnswer(changes), c.answer, changes)
		}
	}
}

func TestAMatchingItemKeyedToAnotherChoiceIsAnAnswer(t *testing.T) {
	changes := onlyChange(t, matchingQuestion(0, 1, 2), matchingQuestion(0, 2, 1))
	wantFields(t, "rekeyed", changes)
	if !hasAnswer(changes) {
		t.Fatalf("rekeying an item is not an answer change: %+v", changes)
	}
}

func TestATFNGAnswerIsAnAnswerChange(t *testing.T) {
	changes := onlyChange(t, tfngQuestion(true, false, false), tfngQuestion(false, false, true))
	wantFields(t, "tfng", changes)
	if !hasAnswer(changes) {
		t.Fatalf("a new tfng answer is not an answer change: %+v", changes)
	}
}

func TestMovingANewTypeQuestionIsStillNotAChange(t *testing.T) {
	m, e := matchingQuestion(0, 1, 2), essayQuestion("Task", "Lexis")
	m.SourceID, e.SourceID = "m", "e"
	if changes := compare(t, onePart(m, e), onePart(e, m)); len(changes) != 0 {
		t.Fatalf("moving questions about reported %+v", changes)
	}
}

func TestAnUnchangedPaperOfNewTypesFindsNothing(t *testing.T) {
	paper := onePart(tfngQuestion(true, false, false), chooseTwo(true, true, false), matchingQuestion(1, 0), essayQuestion("Task"))
	for i := range paper.Sections[0].Questions {
		paper.Sections[0].Questions[i].SourceID = string(rune('a' + i))
	}
	if changes := compare(t, paper, paper); len(changes) != 0 {
		t.Fatalf("comparing a paper with itself reported %+v", changes)
	}
}
