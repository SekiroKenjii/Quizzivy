package domain_test

import (
	"encoding/json"
	"fmt"
	"quizzivy/internal/modules/tests/domain"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func textOf(s string) *string { return &s }

func choiceQuestion(source, prompt, points string, correct ...bool) domain.DraftQuestion {
	q := domain.DraftQuestion{SourceID: source, Type: "single_choice", Prompt: prompt, Points: points}
	for i, isCorrect := range correct {
		q.Options = append(q.Options, domain.DraftOption{Ordinal: i, Text: fmt.Sprintf("option %d of %s", i, prompt), IsCorrect: isCorrect})
	}
	return q
}

func gapDocument(ids ...string) json.RawMessage {
	var inlines []string
	for i, id := range ids {
		inlines = append(inlines, fmt.Sprintf(`{"type":"gap","id":%q,"label":"%d"}`, id, i+1))
	}
	return json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[` + strings.Join(inlines, ",") + `]}]}`)
}

func blankQuestion(source, gap string, answers ...string) domain.DraftQuestion {
	return domain.DraftQuestion{
		SourceID: source, Type: "fill_blank", Prompt: "[1]", Points: "1.00", PromptContent: gapDocument(gap),
		Blanks: []domain.DraftBlank{{Ordinal: 0, GapID: textOf(gap), AcceptedAnswers: answers}},
	}
}

func part(title string, questions ...domain.DraftQuestion) domain.DraftSection {
	return domain.DraftSection{Title: title, Questions: questions}
}

func paperOf(sections ...domain.DraftSection) domain.DraftContent {
	return domain.DraftContent{Sections: sections}
}

func onePart(questions ...domain.DraftQuestion) domain.DraftContent {
	return paperOf(part("Phần 1", questions...))
}

func compare(t *testing.T, from, to domain.DraftContent) []domain.Change {
	t.Helper()
	changes, err := domain.Compare(from, to)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	return changes
}

func introduce(t *testing.T, to domain.DraftContent) []domain.Change {
	t.Helper()
	changes, err := domain.Introduction(to)
	if err != nil {
		t.Fatalf("Introduction: %v", err)
	}
	return changes
}

func kinds(changes []domain.Change) []domain.ChangeKind {
	out := make([]domain.ChangeKind, len(changes))
	for i, c := range changes {
		out[i] = c.Kind
	}
	return out
}

func only(t *testing.T, changes []domain.Change, kind domain.ChangeKind) domain.Change {
	t.Helper()
	var found []domain.Change
	for _, c := range changes {
		if c.Kind == kind {
			found = append(found, c)
		}
	}
	if len(found) != 1 {
		t.Fatalf("changes %v hold %d of kind %s, want 1", kinds(changes), len(found), kind)
	}
	return found[0]
}

func TestComparingAPaperWithItselfFindsNothing(t *testing.T) {
	p := onePart(choiceQuestion("q1", "Một", "1.00", true, false), blankQuestion("q2", "gap-a", "x", "y"))
	if changes := compare(t, p, p); len(changes) != 0 {
		t.Fatalf("changes = %+v, want none", changes)
	}
}

func TestAQuestionOnlyInTheSecondPaperIsAdded(t *testing.T) {
	from := onePart(choiceQuestion("q1", "Một", "1.00", true, false))
	to := onePart(choiceQuestion("q1", "Một", "1.00", true, false), choiceQuestion("q2", "Hai", "2.00", false, true))
	changes := compare(t, from, to)
	added := only(t, changes, domain.ChangeAdded)
	if added.Number != 2 || added.QuestionID != "q2" || added.Prompt != "Hai" {
		t.Errorf("added = %+v, want question 2, q2, \"Hai\"", added)
	}
	points := only(t, changes, domain.ChangePoints)
	if points.PointsFrom != "1.00" || points.PointsTo != "3.00" {
		t.Errorf("points = %+v, want 1.00 to 3.00", points)
	}
	if len(changes) != 2 {
		t.Errorf("changes = %v, want added and points only", kinds(changes))
	}
}

func TestAQuestionOnlyInTheFirstPaperIsRemovedAndNumberedInThatPaper(t *testing.T) {
	from := onePart(choiceQuestion("q1", "Một", "1.00", true, false), choiceQuestion("q2", "Hai", "1.00", true, false), choiceQuestion("q3", "Ba", "1.00", true, false))
	to := onePart(choiceQuestion("q1", "Một", "1.00", true, false), choiceQuestion("q3", "Ba", "1.00", true, false))
	removed := only(t, compare(t, from, to), domain.ChangeRemoved)
	if removed.Number != 2 || removed.QuestionID != "q2" || removed.Prompt != "Hai" {
		t.Errorf("removed = %+v, want question 2 of the first paper", removed)
	}
}

func TestEveryPartOfAQuestionHasItsOwnField(t *testing.T) {
	base := func() domain.DraftQuestion {
		q := choiceQuestion("q1", "Một", "1.00", true, false)
		q.PromptContent = json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"Một","marks":[]}]}]}`)
		q.Explanation = textOf("Vì sao")
		q.MediaAssetID = textOf("0195a000-0000-7000-8000-000000000001")
		q.MaxPlays = new(int)
		q.AllowSeek = new(bool)
		q.ShowTranscript = new(bool)
		q.Transcript = textOf("Lời thoại")
		return q
	}
	cases := []struct {
		name   string
		edit   func(*domain.DraftQuestion)
		fields []domain.ChangedField
	}{
		{"type", func(q *domain.DraftQuestion) { q.Type = "multiple_choice" }, []domain.ChangedField{domain.FieldType}},
		{"prompt text", func(q *domain.DraftQuestion) { q.Prompt = "Một khác" }, []domain.ChangedField{domain.FieldPrompt}},
		{"prompt content", func(q *domain.DraftQuestion) {
			q.PromptContent = json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"Một","marks":["bold"]}]}]}`)
		}, []domain.ChangedField{domain.FieldPrompt}},
		{"option text", func(q *domain.DraftQuestion) { q.Options[1].Text = "khác" }, []domain.ChangedField{domain.FieldOptions}},
		{"option content", func(q *domain.DraftQuestion) {
			q.Options[1].Content = json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"x","marks":[]}]}]}`)
		}, []domain.ChangedField{domain.FieldOptions}},
		{"an option added", func(q *domain.DraftQuestion) {
			q.Options = append(q.Options, domain.DraftOption{Ordinal: 2, Text: "thêm"})
		}, []domain.ChangedField{domain.FieldOptions}},
		{"media asset", func(q *domain.DraftQuestion) { q.MediaAssetID = textOf("0195a000-0000-7000-8000-000000000002") }, []domain.ChangedField{domain.FieldMedia}},
		{"media removed", func(q *domain.DraftQuestion) { q.MediaAssetID = nil }, []domain.ChangedField{domain.FieldMedia}},
		{"audio plays", func(q *domain.DraftQuestion) { plays := 3; q.MaxPlays = &plays }, []domain.ChangedField{domain.FieldMedia}},
		{"audio seek", func(q *domain.DraftQuestion) { seek := true; q.AllowSeek = &seek }, []domain.ChangedField{domain.FieldMedia}},
		{"transcript", func(q *domain.DraftQuestion) { q.Transcript = textOf("Lời khác") }, []domain.ChangedField{domain.FieldMedia}},
		{"explanation", func(q *domain.DraftQuestion) { q.Explanation = textOf("Vì lý do khác") }, []domain.ChangedField{domain.FieldExplanation}},
		{"explanation removed", func(q *domain.DraftQuestion) { q.Explanation = nil }, []domain.ChangedField{domain.FieldExplanation}},
		{"explanation content", func(q *domain.DraftQuestion) {
			q.ExplanationContent = json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"Vì","marks":[]}]}]}`)
		}, []domain.ChangedField{domain.FieldExplanation}},
		{"points", func(q *domain.DraftQuestion) { q.Points = "2.50" }, []domain.ChangedField{domain.FieldPoints}},
		{"prompt and points", func(q *domain.DraftQuestion) { q.Prompt = "Khác"; q.Points = "3.00" }, []domain.ChangedField{domain.FieldPrompt, domain.FieldPoints}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			edited := base()
			c.edit(&edited)
			changed := only(t, compare(t, onePart(base()), onePart(edited)), domain.ChangeChanged)
			if !reflect.DeepEqual(changed.Fields, c.fields) {
				t.Errorf("fields = %v, want %v", changed.Fields, c.fields)
			}
			if changed.Number != 1 || changed.QuestionID != "q1" {
				t.Errorf("changed names question %d %q, want 1 q1", changed.Number, changed.QuestionID)
			}
		})
	}
}

func TestTheCaseRuleAndTheGapOfABlankAreTheBlanksNotTheAnswer(t *testing.T) {
	from := onePart(blankQuestion("q1", "gap-a", "x"))
	caseRule := blankQuestion("q1", "gap-a", "x")
	caseRule.Blanks[0].CaseSensitive = true
	changes := compare(t, from, onePart(caseRule))
	if !reflect.DeepEqual(only(t, changes, domain.ChangeChanged).Fields, []domain.ChangedField{domain.FieldBlanks}) || len(changes) != 1 {
		t.Errorf("changes = %+v, want the blanks field only", changes)
	}
}

func TestTheAnswerKeyIsReportedOnceAndNamesTheOptionLabels(t *testing.T) {
	from := onePart(choiceQuestion("q1", "Một", "1.00", false, true, false))
	to := onePart(choiceQuestion("q1", "Một", "1.00", true, false, false))
	changes := compare(t, from, to)
	answer := only(t, changes, domain.ChangeAnswer)
	if !reflect.DeepEqual(answer.AnswerFrom, []string{"B"}) || !reflect.DeepEqual(answer.AnswerTo, []string{"A"}) {
		t.Errorf("answer = %+v, want B to A", answer)
	}
	if len(changes) != 1 {
		t.Errorf("changes = %v, want the answer alone: the options did not change", kinds(changes))
	}
}

func TestAnAcceptedAnswerOrASampleAnswerIsAnAnswerWithoutLabels(t *testing.T) {
	short := func(sample string) domain.DraftQuestion {
		return domain.DraftQuestion{SourceID: "q2", Type: "short_answer", Prompt: "Hai", Points: "1.00", SampleAnswer: textOf(sample)}
	}
	cases := []struct {
		name     string
		from, to domain.DraftQuestion
	}{
		{"accepted answer added", blankQuestion("q1", "gap-a", "x"), blankQuestion("q1", "gap-a", "x", "y")},
		{"accepted answer replaced", blankQuestion("q1", "gap-a", "x"), blankQuestion("q1", "gap-a", "z")},
		{"sample answer", short("one"), short("two")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			changes := compare(t, onePart(c.from), onePart(c.to))
			answer := only(t, changes, domain.ChangeAnswer)
			if answer.AnswerFrom != nil || answer.AnswerTo != nil {
				t.Errorf("answer = %+v, want no labels for a question without options", answer)
			}
			if len(changes) != 1 {
				t.Errorf("changes = %v, want the answer alone", kinds(changes))
			}
		})
	}
}

func TestTheOrderOfAcceptedAnswersIsNotTheAnswer(t *testing.T) {
	if changes := compare(t, onePart(blankQuestion("q1", "g", "x", "y")), onePart(blankQuestion("q1", "g", "y", "x"))); len(changes) != 0 {
		t.Fatalf("changes = %+v, want none", changes)
	}
}

func TestAWordingChangeAndAKeyChangeAreTwoEntries(t *testing.T) {
	from := onePart(choiceQuestion("q1", "Một", "1.00", true, false))
	to := onePart(choiceQuestion("q1", "Một khác", "1.00", false, true))
	got := kinds(compare(t, from, to))
	want := []domain.ChangeKind{domain.ChangeChanged, domain.ChangeAnswer}
	if !slices.Equal(got, want) {
		t.Fatalf("kinds = %v, want %v", got, want)
	}
}

func TestTheTotalChangesOnceHoweverManyQuestionsDid(t *testing.T) {
	from := onePart(choiceQuestion("q1", "Một", "1.00", true, false), choiceQuestion("q2", "Hai", "1.00", true, false))
	to := onePart(choiceQuestion("q1", "Một", "2.00", true, false), choiceQuestion("q2", "Hai", "3.00", true, false))
	changes := compare(t, from, to)
	want := []domain.ChangeKind{domain.ChangeChanged, domain.ChangeChanged, domain.ChangePoints}
	if !slices.Equal(kinds(changes), want) {
		t.Fatalf("kinds = %v, want %v", kinds(changes), want)
	}
	if total := changes[2]; total.PointsFrom != "2.00" || total.PointsTo != "5.00" || total.Number != 0 || total.QuestionID != "" {
		t.Errorf("total = %+v, want 2.00 to 5.00 and no question", total)
	}
}

func TestPointsThatReadDifferentlyButAreEqualAreNotAChange(t *testing.T) {
	if changes := compare(t, onePart(choiceQuestion("q1", "Một", "1", true, false)), onePart(choiceQuestion("q1", "Một", "1.00", true, false))); len(changes) != 0 {
		t.Fatalf("changes = %+v, want none", changes)
	}
}

func TestMovingQuestionsAboutIsNotAChange(t *testing.T) {
	a := choiceQuestion("q1", "Một", "1.00", true, false)
	b := choiceQuestion("q2", "Hai", "1.00", false, true)
	c := blankQuestion("q3", "gap-a", "x")
	cases := []struct {
		name     string
		from, to domain.DraftContent
	}{
		{"reversed in a section", onePart(a, b, c), onePart(c, b, a)},
		{"sections swapped", paperOf(part("Một", a), part("Hai", b)), paperOf(part("Hai", b), part("Một", a))},
		{"first to last", onePart(a, b, c), onePart(b, c, a)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if changes := compare(t, c.from, c.to); len(changes) != 0 {
				t.Fatalf("changes = %+v, want none", changes)
			}
		})
	}
}

func TestMovingAQuestionToAnotherSectionIsASectionChange(t *testing.T) {
	a := choiceQuestion("q1", "Một", "1.00", true, false)
	b := choiceQuestion("q2", "Hai", "1.00", false, true)
	from := paperOf(part("Đọc", a, b), part("Nghe", choiceQuestion("q3", "Ba", "1.00", true, false)))
	to := paperOf(part("Đọc", a), part("Nghe", choiceQuestion("q3", "Ba", "1.00", true, false), b))
	changed := only(t, compare(t, from, to), domain.ChangeChanged)
	if changed.QuestionID != "q2" || !reflect.DeepEqual(changed.Fields, []domain.ChangedField{domain.FieldSection}) {
		t.Errorf("changed = %+v, want q2 and the section field", changed)
	}
}

func TestRenamingASectionOrChangingItsInstructionsChangesItsQuestions(t *testing.T) {
	a := choiceQuestion("q1", "Một", "1.00", true, false)
	b := choiceQuestion("q2", "Hai", "1.00", true, false)
	renamed := part("Phần đọc", a, b)
	instructed := part("Phần 1", a, b)
	instructed.Instructions = textOf("Làm trong 20 phút")
	for name, to := range map[string]domain.DraftContent{"renamed": paperOf(renamed), "instructions": paperOf(instructed)} {
		t.Run(name, func(t *testing.T) {
			changes := compare(t, onePart(a, b), to)
			if len(changes) != 2 || changes[0].Kind != domain.ChangeChanged || !reflect.DeepEqual(changes[0].Fields, []domain.ChangedField{domain.FieldSection}) {
				t.Fatalf("changes = %+v, want both questions changed in their section", changes)
			}
		})
	}
}

func TestACopyOfAQuestionWithIdenticalContentIsTheSameQuestion(t *testing.T) {
	original := blankQuestion("bank-1", "gap-old", "x", "y")
	original.Explanation = textOf("Vì")
	restored := blankQuestion("bank-2", "gap-new", "y", "x")
	restored.Explanation = textOf("Vì")
	if changes := compare(t, onePart(original), onePart(restored)); len(changes) != 0 {
		t.Fatalf("changes = %+v, want none: restoring a version copies its questions", changes)
	}
}

func TestACopyThatWasEditedReadsAsAddedAndRemoved(t *testing.T) {
	original := blankQuestion("bank-1", "gap-old", "x")
	restored := blankQuestion("bank-2", "gap-new", "x", "z")
	got := kinds(compare(t, onePart(original), onePart(restored)))
	if want := []domain.ChangeKind{domain.ChangeAdded, domain.ChangeRemoved}; !slices.Equal(got, want) {
		t.Fatalf("kinds = %v, want %v: an edited copy is not matched to its original", got, want)
	}
}

func TestAGapMovedToAnotherPlaceInThePromptIsABlanksChange(t *testing.T) {
	from := domain.DraftQuestion{SourceID: "q1", Type: "fill_blank", Prompt: "[1] [2]", Points: "1.00", PromptContent: gapDocument("a", "b"),
		Blanks: []domain.DraftBlank{{Ordinal: 0, GapID: textOf("a"), AcceptedAnswers: []string{"x"}}, {Ordinal: 1, GapID: textOf("b"), AcceptedAnswers: []string{"y"}}}}
	to := from
	to.Blanks = []domain.DraftBlank{{Ordinal: 0, GapID: textOf("b"), AcceptedAnswers: []string{"x"}}, {Ordinal: 1, GapID: textOf("a"), AcceptedAnswers: []string{"y"}}}
	changed := only(t, compare(t, onePart(from), onePart(to)), domain.ChangeChanged)
	if !reflect.DeepEqual(changed.Fields, []domain.ChangedField{domain.FieldBlanks}) {
		t.Errorf("fields = %v, want blanks", changed.Fields)
	}
}

func TestQuestionsWithTheSameSourcePairInPaperOrder(t *testing.T) {
	same := func(prompt string) domain.DraftQuestion { return choiceQuestion("bank-1", prompt, "1.00", true, false) }
	from := paperOf(part("A", same("đầu")), part("B", same("sau")))
	to := paperOf(part("A", same("đầu")), part("B", same("sau")))
	if changes := compare(t, from, to); len(changes) != 0 {
		t.Fatalf("changes = %+v, want none", changes)
	}
}

func TestAFrozenQuestionIsNamedByItsFrozenRow(t *testing.T) {
	frozen := choiceQuestion("bank-1", "Một", "1.00", true, false)
	frozen.FrozenID = "frozen-1"
	draft := choiceQuestion("bank-1", "Một khác", "1.00", true, false)
	removed := choiceQuestion("bank-2", "Hai", "1.00", true, false)
	removed.FrozenID = "frozen-2"
	changes := compare(t, paperOf(part("A", frozen, removed)), onePart(draft))
	if got := only(t, changes, domain.ChangeChanged).QuestionID; got != "bank-1" {
		t.Errorf("changed id = %q, want the draft's bank question", got)
	}
	if got := only(t, changes, domain.ChangeRemoved).QuestionID; got != "frozen-2" {
		t.Errorf("removed id = %q, want the frozen row of the first paper", got)
	}
	back := compare(t, onePart(draft), paperOf(part("A", frozen, removed)))
	if got := only(t, back, domain.ChangeChanged).QuestionID; got != "frozen-1" {
		t.Errorf("changed id = %q, want the second paper's frozen row", got)
	}
}

func TestChangesComeByKindThenByQuestionNumber(t *testing.T) {
	from := onePart(
		choiceQuestion("q1", "Một", "1.00", true, false),
		choiceQuestion("q2", "Hai", "1.00", true, false),
		choiceQuestion("q3", "Ba", "1.00", true, false),
		choiceQuestion("q4", "Bốn", "1.00", true, false),
	)
	to := onePart(
		choiceQuestion("q1", "Một", "1.00", false, true),
		choiceQuestion("q2", "Hai khác", "1.00", true, false),
		choiceQuestion("q4", "Bốn", "1.00", true, false),
		choiceQuestion("q5", "Năm", "2.00", true, false),
		choiceQuestion("q6", "Sáu", "2.00", true, false),
	)
	changes := compare(t, from, to)
	want := []domain.ChangeKind{
		domain.ChangeAdded, domain.ChangeAdded, domain.ChangeRemoved, domain.ChangeChanged, domain.ChangeAnswer, domain.ChangePoints,
	}
	if !slices.Equal(kinds(changes), want) {
		t.Fatalf("kinds = %v, want %v", kinds(changes), want)
	}
	if changes[0].Number != 4 || changes[1].Number != 5 {
		t.Errorf("added numbers = %d, %d, want 4 and 5", changes[0].Number, changes[1].Number)
	}
}

func TestAFirstVersionIsEveryQuestionAddedAndNoTotal(t *testing.T) {
	changes := introduce(t, paperOf(
		part("A", choiceQuestion("q1", "Một", "1.00", true, false)),
		part("B", choiceQuestion("q2", "Hai", "2.00", true, false)),
	))
	if len(changes) != 2 {
		t.Fatalf("changes = %+v, want two added", changes)
	}
	for i, c := range changes {
		if c.Kind != domain.ChangeAdded || c.Number != i+1 {
			t.Errorf("change %d = %+v, want added question %d", i, c, i+1)
		}
	}
}

func TestAnEmptyDraftRemovesEverythingAndMovesTheTotal(t *testing.T) {
	changes := compare(t, onePart(choiceQuestion("q1", "Một", "1.00", true, false)), domain.DraftContent{})
	want := []domain.ChangeKind{domain.ChangeRemoved, domain.ChangePoints}
	if !slices.Equal(kinds(changes), want) || changes[1].PointsTo != "0.00" {
		t.Fatalf("changes = %+v, want removed and the total down to 0.00", changes)
	}
}

func TestALongPromptIsCutToTheLimitWithAnEllipsis(t *testing.T) {
	long := strings.Repeat("ế", domain.MaxChangePrompt+50)
	short := strings.Repeat("ế", domain.MaxChangePrompt)
	added := only(t, introduce(t, onePart(choiceQuestion("q1", long, "1.00", true, false))), domain.ChangeAdded)
	if got := []rune(added.Prompt); len(got) != domain.MaxChangePrompt || got[len(got)-1] != '…' {
		t.Errorf("prompt has %d characters ending %q, want %d ending …", len(got), string(got[len(got)-1]), domain.MaxChangePrompt)
	}
	exact := only(t, introduce(t, onePart(choiceQuestion("q1", short, "1.00", true, false))), domain.ChangeAdded)
	if exact.Prompt != short {
		t.Errorf("a prompt at the limit was cut")
	}
}

func swapOptions(q *domain.DraftQuestion) {
	q.Options[0], q.Options[1] = q.Options[1], q.Options[0]
	q.Options[0].Ordinal, q.Options[1].Ordinal = 0, 1
}

func TestTheAnswerKeyIsReadByPositionLikeTheLettersTheTeacherSees(t *testing.T) {
	from := onePart(choiceQuestion("q1", "Một", "1.00", true, false))

	keptOnTheFirstLetter := onePart(choiceQuestion("q1", "Một", "1.00", false, true))
	swapOptions(&keptOnTheFirstLetter.Sections[0].Questions[0])
	if got := kinds(compare(t, from, keptOnTheFirstLetter)); !slices.Equal(got, []domain.ChangeKind{domain.ChangeChanged}) {
		t.Errorf("a key that stays on A: kinds = %v, want the options only", got)
	}

	movedWithItsOption := onePart(choiceQuestion("q1", "Một", "1.00", true, false))
	swapOptions(&movedWithItsOption.Sections[0].Questions[0])
	changes := compare(t, from, movedWithItsOption)
	if got := kinds(changes); !slices.Equal(got, []domain.ChangeKind{domain.ChangeChanged, domain.ChangeAnswer}) {
		t.Fatalf("a key that moves with its option: kinds = %v, want the options and the answer", got)
	}
	if answer := changes[1]; !reflect.DeepEqual(answer.AnswerFrom, []string{"A"}) || !reflect.DeepEqual(answer.AnswerTo, []string{"B"}) {
		t.Errorf("answer = %+v, want A to B", answer)
	}
}

func groupedPaper(passage string, ids [4]string, optionOrder string) domain.DraftContent {
	choice := choiceQuestion(ids[0], "Chọn", "1.00", true, false)
	blank := blankQuestion(ids[1], ids[3], "x")
	material := domain.GroupStimulus{
		ID: ids[2], Title: "Bài đọc",
		Content: json.RawMessage(fmt.Sprintf(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":%q,"marks":[]},{"type":"gap","id":"m-%s","label":"1"}]}]}`, passage, ids[3])),
		Gaps: []domain.GroupGapBinding{
			{Kind: "question", GapID: "m-" + ids[3], QuestionID: ids[0]},
		},
	}
	material.Gaps = append(material.Gaps, domain.GroupGapBinding{Kind: "blank", GapID: "m-" + ids[3], QuestionID: ids[1], BlankGapID: textOf(ids[3])})
	group := domain.GroupBundle{Group: domain.QuestionGroup{
		ID: "group-" + ids[2], Title: "Nhóm",
		Members: []domain.GroupMember{{QuestionID: ids[0], OptionOrder: optionOrder}, {QuestionID: ids[1], OptionOrder: "shuffle"}},
		Stimuli: []domain.GroupStimulus{material}, Recordings: []domain.GroupRecording{},
	}}
	section := part("Phần đọc", choice, blank)
	section.Groups = []domain.GroupBundle{group}
	return paperOf(section)
}

func TestAGroupRestoredWithEveryIdRenewedIsTheSameGroup(t *testing.T) {
	original := groupedPaper("Đoạn văn", [4]string{"q1", "q2", "stim-1", "gap-1"}, "fixed")
	restored := groupedPaper("Đoạn văn", [4]string{"q9", "q8", "stim-9", "gap-9"}, "fixed")
	if changes := compare(t, original, restored); len(changes) != 0 {
		t.Fatalf("changes = %+v, want none", changes)
	}
}

func TestEditingASharedPassageChangesThePassageOfEveryMember(t *testing.T) {
	original := groupedPaper("Đoạn văn", [4]string{"q1", "q2", "stim-1", "gap-1"}, "fixed")
	edited := groupedPaper("Đoạn văn đã sửa", [4]string{"q1", "q2", "stim-1", "gap-1"}, "fixed")
	changes := compare(t, original, edited)
	if len(changes) != 2 {
		t.Fatalf("changes = %+v, want one per member", changes)
	}
	for _, c := range changes {
		if c.Kind != domain.ChangeChanged || !reflect.DeepEqual(c.Fields, []domain.ChangedField{domain.FieldContext}) {
			t.Errorf("change = %+v, want the context field", c)
		}
	}
}

func TestAMemberOptionOrderChangesThatMemberOnly(t *testing.T) {
	original := groupedPaper("Đoạn văn", [4]string{"q1", "q2", "stim-1", "gap-1"}, "fixed")
	edited := groupedPaper("Đoạn văn", [4]string{"q1", "q2", "stim-1", "gap-1"}, "shuffle")
	changes := compare(t, original, edited)
	if len(changes) != 1 || changes[0].QuestionID != "q1" || !reflect.DeepEqual(changes[0].Fields, []domain.ChangedField{domain.FieldContext}) {
		t.Fatalf("changes = %+v, want the context of q1 alone", changes)
	}
}

func TestAQuestionLeavingItsGroupChangesItsContext(t *testing.T) {
	original := groupedPaper("Đoạn văn", [4]string{"q1", "q2", "stim-1", "gap-1"}, "fixed")
	ungrouped := original
	ungrouped.Sections = []domain.DraftSection{part("Phần đọc", original.Sections[0].Questions...)}
	changes := compare(t, original, ungrouped)
	if len(changes) != 2 || changes[0].Kind != domain.ChangeChanged || !reflect.DeepEqual(changes[0].Fields, []domain.ChangedField{domain.FieldContext}) {
		t.Fatalf("changes = %+v, want the context of both members", changes)
	}
}

func TestAnAddedMemberDoesNotChangeTheContextOfTheOthers(t *testing.T) {
	original := groupedPaper("Đoạn văn", [4]string{"q1", "q2", "stim-1", "gap-1"}, "fixed")
	extended := groupedPaper("Đoạn văn", [4]string{"q1", "q2", "stim-1", "gap-1"}, "fixed")
	extra := choiceQuestion("q3", "Thêm", "1.00", true, false)
	extended.Sections[0].Questions = append(extended.Sections[0].Questions, extra)
	extended.Sections[0].Groups[0].Group.Members = append(extended.Sections[0].Groups[0].Group.Members, domain.GroupMember{QuestionID: "q3", OptionOrder: "shuffle"})
	got := kinds(compare(t, original, extended))
	if want := []domain.ChangeKind{domain.ChangeAdded, domain.ChangePoints}; !slices.Equal(got, want) {
		t.Fatalf("kinds = %v, want %v: a new member is its own entry and leaves the others' context alone", got, want)
	}
}

func TestARecordingPolicyAndTranscriptBelongToTheGroup(t *testing.T) {
	recorded := func(plays int, transcript string) domain.DraftContent {
		p := groupedPaper("Đoạn văn", [4]string{"q1", "q2", "stim-1", "gap-1"}, "fixed")
		p.Sections[0].Groups[0].Group.Recordings = []domain.GroupRecording{{ID: "rec", AssetID: "0195A000-0000-7000-8000-000000000001", Transcript: textOf(transcript)}}
		p.Sections[0].Groups[0].Group.Recordings[0].Policy.MaxPlays = &plays
		return p
	}
	if changes := compare(t, recorded(2, "a"), recorded(2, "a")); len(changes) != 0 {
		t.Fatalf("changes = %+v, want none", changes)
	}
	if changes := compare(t, recorded(2, "a"), recorded(3, "a")); len(changes) != 2 {
		t.Errorf("a different play limit: changes = %+v, want the context of both members", changes)
	}
	if changes := compare(t, recorded(2, "a"), recorded(2, "b")); len(changes) != 2 {
		t.Errorf("a different transcript: changes = %+v, want the context of both members", changes)
	}
}

func TestAScoreThatIsNotOneFailsTheComparisonInsteadOfCountingAsZero(t *testing.T) {
	good := onePart(choiceQuestion("q1", "Một", "1.00", true, false))
	for name, points := range map[string]string{"words": "many", "empty": "", "zero": "0.00", "too many decimals": "1.005", "negative": "-1"} {
		t.Run(name, func(t *testing.T) {
			bad := onePart(choiceQuestion("q1", "Một", points, true, false))
			if changes, err := domain.Compare(good, bad); err == nil {
				t.Errorf("Compare(good, bad) = %+v, want an error for points %q", changes, points)
			}
			if changes, err := domain.Compare(bad, good); err == nil {
				t.Errorf("Compare(bad, good) = %+v, want an error for points %q", changes, points)
			}
			if changes, err := domain.Introduction(bad); err == nil {
				t.Errorf("Introduction(bad) = %+v, want an error for points %q", changes, points)
			}
		})
	}
}
