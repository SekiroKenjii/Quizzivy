package domain_test

import (
	"encoding/json"
	"errors"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/shared/content"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func prose(text string) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{"format": "semantic_v1", "blocks": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": text, "marks": []string{}}}}}})
	return raw
}

func underlinedOption(before, marked string) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{"format": "semantic_v1", "blocks": []any{map[string]any{"type": "paragraph", "content": []any{
		map[string]any{"type": "text", "text": before, "marks": []string{}},
		map[string]any{"type": "text", "text": marked, "marks": []string{"underline"}},
	}}}})
	return raw
}

func gapped(before, id, label, after string) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{"format": "semantic_v1", "blocks": []any{map[string]any{"type": "paragraph", "content": []any{
		map[string]any{"type": "text", "text": before, "marks": []string{}},
		map[string]any{"type": "gap", "id": id, "label": label},
		map[string]any{"type": "text", "text": after, "marks": []string{}},
	}}}})
	return raw
}

func choice(id string, correct ...string) domain.DraftQuestion {
	q := domain.DraftQuestion{ID: id, Label: id, Type: "single_choice", Prompt: prose("Pick one " + id), Points: "1", Blanks: []domain.DraftBlank{}, Source: []domain.SourceRef{}}
	q.Origins = domain.Origins{Type: domain.InferredStructure, Prompt: domain.SourceExplicit, Options: domain.SourceExplicit, Answer: domain.SourceExplicit, Points: domain.Defaulted}
	for _, label := range []string{"A", "B", "C", "D"} {
		q.Options = append(q.Options, domain.DraftOption{ID: id + "-" + label, Label: label, Content: prose("option " + label)})
	}
	q.Answer = domain.DraftAnswer{State: domain.AnswerUnknown, OptionIDs: []string{}, Evidence: []domain.SourceRef{}}
	if len(correct) > 0 {
		q.Answer.State = domain.AnswerKnown
		for _, label := range correct {
			q.Answer.OptionIDs = append(q.Answer.OptionIDs, id+"-"+label)
		}
	}
	return q
}

func draft(items ...domain.DraftItem) domain.Draft {
	return domain.Draft{Version: domain.DraftVersion, Title: "TEST 1", Sections: []domain.DraftSection{{ID: "s1", Title: "I. Choose", Origin: domain.SourceExplicit, Items: items, Source: []domain.SourceRef{}}}, Notices: []domain.Finding{}, Acknowledged: []string{}}
}

func questionItem(q domain.DraftQuestion) domain.DraftItem { return domain.DraftItem{Question: &q} }

func codes(r domain.Review) []string {
	var out []string
	for _, f := range r.Findings {
		if f.Severity != domain.Informational {
			out = append(out, f.Code)
		}
	}
	slices.Sort(out)
	return out
}

func finding(r domain.Review, code string) domain.Finding {
	for _, f := range r.Findings {
		if f.Code == code {
			return f
		}
	}
	return domain.Finding{}
}

func TestAnUnknownChoiceKeyBlocksUntilTheTeacherSetsIt(t *testing.T) {
	d := draft(questionItem(choice("q1")), questionItem(choice("q2", "B")))
	r := domain.Assess(d)
	if r.Ready || !slices.Equal(codes(r), []string{domain.CodeMissingAnswer}) || r.Summary.AnswersMissing != 1 {
		t.Fatalf("review %+v", r)
	}
	fixed := choice("q1", "C")
	fixed.Origins.Answer = domain.TeacherEntered
	d.Sections[0].Items[0] = questionItem(fixed)
	if r := domain.Assess(d); !r.Ready || r.Summary.TotalPoints != "2.00" {
		t.Fatalf("review %+v", r)
	}
}

func TestConflictingKeysBlockAndCannotBeAcknowledgedAway(t *testing.T) {
	q := choice("q1")
	q.Answer.State = domain.AnswerConflict
	q.Answer.Candidates = []domain.KeyValue{{Value: "A", Evidence: []domain.SourceRef{}}, {Value: "B", Evidence: []domain.SourceRef{}}}
	d := draft(questionItem(q))
	r := domain.Assess(d)
	f := finding(r, domain.CodeConflictingKeys)
	d.Acknowledged = []string{f.ID}
	if r := domain.Assess(d); r.Ready || f.Severity != domain.Blocking {
		t.Fatalf("blocker acknowledged away: %+v", r)
	}
}

func TestUnsupportedContentMustBeExcludedAndExclusionChangesTotals(t *testing.T) {
	matching := choice("q1")
	matching.Type = domain.UnsupportedType
	d := draft(questionItem(matching), questionItem(choice("q2", "A")))
	if r := domain.Assess(d); r.Ready || !slices.Equal(codes(r), []string{domain.CodeUnsupportedInteraction}) {
		t.Fatalf("review %+v", codes(r))
	}
	matching.Excluded = &domain.Exclusion{Reason: "Matching is not supported"}
	d.Sections[0].Items[0] = questionItem(matching)
	r := domain.Assess(d)
	if !r.Ready || r.Summary.Included != 1 || r.Summary.Excluded != 1 || r.Summary.TotalPoints != "1.00" {
		t.Fatalf("review %+v", r.Summary)
	}
}

func TestSourceNoticesNeedAnAcknowledgementBeforeCommit(t *testing.T) {
	d := draft(questionItem(choice("q1", "A")))
	d.Notices = []domain.Finding{{ID: "n1", Code: domain.CodeUnassignedText, Severity: domain.ReviewRequired, Count: 2, Evidence: []domain.SourceRef{}}}
	if r := domain.Assess(d); r.Ready || r.Summary.NeedsDecision != 1 {
		t.Fatalf("review %+v", r.Summary)
	}
	d.Acknowledged = []string{"n1"}
	if r := domain.Assess(d); !r.Ready || !finding(r, domain.CodeUnassignedText).Acknowledged {
		t.Fatalf("review %+v", r)
	}
}

func TestAMissingPronunciationUnderlineIsAskedAgainAfterTheOptionsChange(t *testing.T) {
	q := choice("q1", "A")
	q.Task = "pronunciation"
	d := draft(questionItem(q))
	first := finding(domain.Assess(d), domain.CodeMissingUnderline)
	d.Acknowledged = []string{first.ID}
	if !domain.Assess(d).Ready {
		t.Fatal("acknowledged underline decision ignored")
	}
	q.Options[1].Content = prose("changed")
	d.Sections[0].Items[0] = questionItem(q)
	if domain.Assess(d).Ready {
		t.Fatal("stale acknowledgement survived an options edit")
	}
	q.Options[0].Content = underlinedOption("tr", "u")
	d.Sections[0].Items[0] = questionItem(q)
	if len(codes(domain.Assess(d))) != 0 {
		t.Fatal("underline added but still reported")
	}
}

func TestOptionsThatNameOtherOptionsAreFlaggedForShuffling(t *testing.T) {
	q := choice("q1", "D")
	q.Options[3].Content = prose("B and C are correct")
	if got := codes(domain.Assess(draft(questionItem(q)))); !slices.Equal(got, []string{domain.CodeOptionReference}) {
		t.Fatalf("codes %v", got)
	}
}

func cloze() domain.DraftGroup {
	first, second := choice("c1", "A"), choice("c2", "B")
	first.Prompt, second.Prompt = prose("(1)"), prose("(2)")
	return domain.DraftGroup{ID: "g1", Stimulus: json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"It is ","marks":[]},{"type":"gap","id":"gap-1","label":"1"},{"type":"text","text":" & ","marks":[]},{"type":"gap","id":"gap-2","label":"2"}]}]}`),
		Gaps: []domain.GapLink{{GapID: "gap-1", QuestionID: "c1"}, {GapID: "gap-2", QuestionID: "c2"}}, Questions: []domain.DraftQuestion{first, second}, Source: []domain.SourceRef{}}
}

func TestPlanGivesFreshIdentitiesSoTheSameFileCanBeImportedTwice(t *testing.T) {
	g := cloze()
	d := draft(questionItem(choice("q1", "A")), domain.DraftItem{Group: &g})
	first, err := domain.Plan(d, "fallback", uuid.NewString)
	if err != nil {
		t.Fatal(err)
	}
	second, err := domain.Plan(d, "fallback", uuid.NewString)
	if err != nil {
		t.Fatal(err)
	}
	a, b := first.Sections[0].Units[1].Group, second.Sections[0].Units[1].Group
	if a.Group.ID == b.Group.ID || a.Questions[0].ID == b.Questions[0].ID {
		t.Fatal("plans share persistent identities")
	}
	if err := a.Validate(); err != nil || a.Group.Stimuli[0].Gaps[0].QuestionID != a.Questions[0].ID || a.Group.Members[0].OptionOrder != "fixed" {
		t.Fatalf("invalid bundle %v %+v", err, a.Group)
	}
	if !strings.Contains(string(a.Group.Stimuli[0].Content), " & ") {
		t.Fatalf("stimulus HTML-escaped: %s", a.Group.Stimuli[0].Content)
	}
}

func TestExcludingAClozeMemberLeavesItsPrintedLabelInThePassage(t *testing.T) {
	g := cloze()
	g.Questions[1].Excluded = &domain.Exclusion{Reason: "duplicate"}
	d := draft(domain.DraftItem{Group: &g})
	plan, err := domain.Plan(d, "fallback", uuid.NewString)
	if err != nil {
		t.Fatal(err)
	}
	bundle := plan.Sections[0].Units[0].Group
	doc, err := content.Parse(bundle.Group.Stimuli[0].Content)
	if err != nil || len(bundle.Questions) != 1 || len(bundle.Group.Stimuli[0].Gaps) != 1 || doc.PlainText() != "It is [1] & (2)" {
		t.Fatalf("stimulus %q gaps %+v err %v", doc.PlainText(), bundle.Group.Stimuli[0].Gaps, err)
	}
	if err := bundle.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestPlanRefusesADraftThatIsNotReady(t *testing.T) {
	if _, err := domain.Plan(draft(questionItem(choice("q1"))), "fallback", uuid.NewString); !errors.Is(err, domain.ErrNotReady) {
		t.Fatalf("err %v", err)
	}
}

func TestPlanMapsBlanksAndSampleAnswers(t *testing.T) {
	fill := domain.DraftQuestion{ID: "f1", Label: "1", Type: "fill_blank", Prompt: gapped("We ", "blank-1", "1", " on holiday"), Options: []domain.DraftOption{}, Points: "0.5", Source: []domain.SourceRef{},
		Blanks: []domain.DraftBlank{{GapID: "blank-1", Accepted: []string{"will go"}}}, Answer: domain.DraftAnswer{State: domain.AnswerKnown, OptionIDs: []string{}, Evidence: []domain.SourceRef{}},
		Origins: domain.Origins{Type: domain.InferredStructure, Prompt: domain.SourceExplicit, Options: domain.SourceExplicit, Answer: domain.SourceExplicit, Points: domain.TeacherEntered}}
	essay := fill
	essay.ID, essay.Type, essay.Prompt, essay.Blanks = "e1", "short_answer", prose("Rewrite it"), []domain.DraftBlank{}
	essay.Answer = domain.DraftAnswer{State: domain.AnswerKnown, OptionIDs: []string{}, Text: "I am fond of TV.", Evidence: []domain.SourceRef{}}
	plan, err := domain.Plan(draft(questionItem(fill), questionItem(essay)), "fallback", uuid.NewString)
	if err != nil {
		t.Fatal(err)
	}
	blank, sample := plan.Sections[0].Units[0].Question, plan.Sections[0].Units[1].Question
	if *blank.Blanks[0].GapID != "blank-1" || blank.Blanks[0].AcceptedAnswers[0] != "will go" || blank.Validate(nil) != nil || *sample.SampleAnswer != "I am fond of TV." {
		t.Fatalf("plan %+v %+v", blank, sample)
	}
}

func TestMalformedEditsAreRejectedButContentProblemsAreNot(t *testing.T) {
	valid := draft(questionItem(choice("q1")))
	if err := domain.ValidateEdit(valid); err != nil {
		t.Fatalf("an unanswered question is a finding, not a malformed edit: %v", err)
	}
	for name, edit := range map[string]func(*domain.Draft){
		"duplicate id":   func(d *domain.Draft) { d.Sections[0].Items = append(d.Sections[0].Items, d.Sections[0].Items[0]) },
		"unknown option": func(d *domain.Draft) { d.Sections[0].Items[0].Question.Answer.OptionIDs = []string{"ghost"} },
		"bad content":    func(d *domain.Draft) { d.Sections[0].Items[0].Question.Prompt = json.RawMessage(`{"format":"html"}`) },
		"both kinds":     func(d *domain.Draft) { g := cloze(); d.Sections[0].Items[0].Group = &g },
		"bad points":     func(d *domain.Draft) { d.Sections[0].Items[0].Question.Points = "-1" },
	} {
		d := draft(questionItem(choice("q1")))
		edit(&d)
		if err := domain.ValidateEdit(d); !errors.Is(err, domain.ErrBadDraft) {
			t.Errorf("%s accepted: %v", name, err)
		}
	}
}

func TestLargeDraftsAssessWithinBudget(t *testing.T) {
	var items []domain.DraftItem
	for i := range 400 {
		items = append(items, questionItem(choice("q"+strconv.Itoa(i), "A")))
	}
	if r := domain.Assess(draft(items...)); !r.Ready || r.Summary.Included != 400 {
		t.Fatalf("summary %+v", r.Summary)
	}
}
