package domain_test

import (
	"errors"
	"quizzivy/internal/modules/imports/domain"
	"testing"
)

func count(n int) *int { return &n }

func tfngQuestion(id string) domain.DraftQuestion {
	q := choice(id, "NG")
	q.Type = "tfng"
	q.Options = []domain.DraftOption{{ID: id + "-T", Label: "T", Content: prose("True")}, {ID: id + "-F", Label: "F", Content: prose("False")}, {ID: id + "-NG", Label: "NG", Content: prose("Not given")}}
	return q
}

func TestEditsAcceptTFNGAndTheNewFieldsWithinTheirBounds(t *testing.T) {
	chooseTwo := choice("q2", "A", "C")
	chooseTwo.Type, chooseTwo.SelectCount = "multiple_choice", count(2)
	limited := choice("q3")
	limited.Type, limited.Options, limited.MaxWords = "short_answer", []domain.DraftOption{}, count(3)
	d := draft(questionItem(tfngQuestion("q1")), questionItem(chooseTwo), questionItem(limited))
	if err := domain.ValidateEdit(d); err != nil {
		t.Fatalf("tfng, selectCount and maxWords are a sound edit: %v", err)
	}
	for name, edit := range map[string]func(*domain.DraftQuestion){
		"selectCount below two": func(q *domain.DraftQuestion) { q.SelectCount = count(1) },
		"selectCount over cap":  func(q *domain.DraftQuestion) { q.SelectCount = count(8) },
		"maxWords zero":         func(q *domain.DraftQuestion) { q.MaxWords = count(0) },
		"maxWords over cap":     func(q *domain.DraftQuestion) { q.MaxWords = count(101) },
	} {
		q := choice("q1", "A")
		edit(&q)
		if err := domain.ValidateEdit(draft(questionItem(q))); !errors.Is(err, domain.ErrBadDraft) {
			t.Errorf("%s accepted: %v", name, err)
		}
	}
}

func TestAFieldOnTheWrongTypeIsAFindingNotARejection(t *testing.T) {
	q := choice("q1", "A")
	q.MaxWords = count(2)
	d := draft(questionItem(q))
	if err := domain.ValidateEdit(d); err != nil {
		t.Fatalf("a misplaced field is a content problem: %v", err)
	}
	if f := finding(domain.Assess(d), domain.CodeInvalidQuestion); f.Field != "maxWords" {
		t.Fatalf("misplaced maxWords: %+v", f)
	}
}

func TestPlanWritesTFNGOptionsAndTheNewFields(t *testing.T) {
	chooseTwo := choice("q2", "A", "C")
	chooseTwo.Type, chooseTwo.SelectCount = "multiple_choice", count(2)
	limited := choice("q3")
	limited.Type, limited.Options, limited.MaxWords = "short_answer", []domain.DraftOption{}, count(3)
	limited.Answer.State, limited.Answer.Text = domain.AnswerKnown, "in Hue"
	plan, err := domain.Plan(draft(questionItem(tfngQuestion("q1")), questionItem(chooseTwo), questionItem(limited)), "", func() string { return "id" })
	if err != nil {
		t.Fatal(err)
	}
	units := plan.Sections[0].Units
	tfng, two, words := units[0].Question, units[1].Question, units[2].Question
	if string(tfng.Type) != "tfng" || len(tfng.Options) != 3 || !tfng.Options[2].IsCorrect || tfng.Options[0].IsCorrect {
		t.Fatalf("tfng input: %+v", tfng)
	}
	if two.SelectCount == nil || *two.SelectCount != 2 || words.MaxWords == nil || *words.MaxWords != 3 {
		t.Fatalf("new fields: %+v %+v", two, words)
	}
}

func TestATFNGGroupMemberKeepsItsOptionsInOrder(t *testing.T) {
	g := domain.DraftGroup{ID: "g1", Label: "1-2", Instructions: "Write TRUE, FALSE or NOT GIVEN.", Gaps: []domain.GapLink{}, Questions: []domain.DraftQuestion{tfngQuestion("q1"), tfngQuestion("q2")}, Source: []domain.SourceRef{}}
	d := draft(domain.DraftItem{Group: &g})
	if r := domain.Assess(d); !r.Ready {
		t.Fatalf("a tfng group is ready: %+v", r.Findings)
	}
	n := 0
	plan, err := domain.Plan(d, "", func() string { n++; return "00000000-0000-4000-8000-00000000000" + string(rune('0'+n)) })
	if err != nil {
		t.Fatal(err)
	}
	for _, member := range plan.Sections[0].Units[0].Group.Group.Members {
		if member.OptionOrder != "fixed" {
			t.Fatalf("tfng options never shuffle: %+v", member)
		}
	}
}
