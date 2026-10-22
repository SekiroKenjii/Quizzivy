package domain_test

import (
	"quizzivy/internal/modules/attempts/domain"
	"testing"
)

type gradeCase struct {
	name    string
	payload string
	want    float64
}

func gradeAll(t *testing.T, q domain.GradableQuestion, cases []gradeCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := domain.Grading.Grade(q, []byte(c.payload))
			if got.Score != c.want || got.RequiresManual {
				t.Errorf("score %v (manual %v), want %v", got.Score, got.RequiresManual, c.want)
			}
		})
	}
}

func selectCount(n int) *int { return &n }

func TestTFNGScoresTheOneCorrectOption(t *testing.T) {
	gradeAll(t, domain.GradableQuestion{Type: "tfng", Points: 1, Options: choice(false, false, true)}, []gradeCase{
		{"empty", `{"type":"choice","optionIds":[]}`, 0},
		{"wrong", `{"type":"choice","optionIds":["a"]}`, 0},
		{"partial is not a thing: right and wrong", `{"type":"choice","optionIds":["a","c"]}`, 0},
		{"right", `{"type":"choice","optionIds":["c"]}`, 1},
		{"malformed", `{"type":"choice","optionIds":"c"}`, 0},
	})
}

func TestChooseNPaysPerCorrectPick(t *testing.T) {
	q := domain.GradableQuestion{Type: "multiple_choice", Points: 2, SelectCount: selectCount(2), Options: choice(true, false, true, false, false)}
	gradeAll(t, q, []gradeCase{
		{"empty", `{"type":"choice","optionIds":[]}`, 0},
		{"two wrong", `{"type":"choice","optionIds":["b","d"]}`, 0},
		{"one right, one wrong", `{"type":"choice","optionIds":["a","b"]}`, 1},
		{"one right alone", `{"type":"choice","optionIds":["c"]}`, 1},
		{"both right", `{"type":"choice","optionIds":["c","a"]}`, 2},
		{"both right and a third pick", `{"type":"choice","optionIds":["a","c","b"]}`, 0},
		{"another question's option", `{"type":"choice","optionIds":["a","zz"]}`, 1},
		{"malformed", `{"type":"choice"`, 0},
	})
	third := domain.GradableQuestion{Type: "multiple_choice", Points: 1, SelectCount: selectCount(3), Options: choice(true, true, true, false)}
	gradeAll(t, third, []gradeCase{{"one of three rounds to hundredths", `{"type":"choice","optionIds":["a"]}`, 0.33}})
}

func TestAPlainMultipleChoiceStaysAllOrNothing(t *testing.T) {
	gradeAll(t, domain.GradableQuestion{Type: "multiple_choice", Points: 2, Options: choice(true, false, true)}, []gradeCase{
		{"one of two", `{"type":"choice","optionIds":["a"]}`, 0},
		{"both", `{"type":"choice","optionIds":["a","c"]}`, 2},
	})
}

func TestMatchingPaysPerPairedItem(t *testing.T) {
	q := domain.GradableQuestion{Type: "matching", Points: 3, Options: choice(false, false, false),
		MatchItems: []domain.GradableMatchItem{{ID: "A", OptionID: "c"}, {ID: "B", OptionID: "a"}, {ID: "C", OptionID: "a"}}}
	gradeAll(t, q, []gradeCase{
		{"empty", `{"type":"matching","pairs":{}}`, 0},
		{"all wrong", `{"type":"matching","pairs":{"A":"a","B":"b","C":"c"}}`, 0},
		{"one right", `{"type":"matching","pairs":{"A":"c","B":"b"}}`, 1},
		{"two right", `{"type":"matching","pairs":{"A":"c","B":"a","C":"b"}}`, 2},
		{"all right", `{"type":"matching","pairs":{"A":"c","B":"a","C":"a"}}`, 3},
		{"a pair for an item it does not have", `{"type":"matching","pairs":{"Z":"c"}}`, 0},
		{"malformed", `{"type":"matching","pairs":["c"]}`, 0},
	})
	two := domain.GradableQuestion{Type: "matching", Points: 1, MatchItems: []domain.GradableMatchItem{{ID: "A", OptionID: "a"}, {ID: "B", OptionID: "b"}, {ID: "C", OptionID: "c"}}}
	gradeAll(t, two, []gradeCase{{"two of three rounds to hundredths", `{"type":"matching","pairs":{"A":"a","B":"b"}}`, 0.67}})
}

func TestAnEssayIsLeftForTheTeacher(t *testing.T) {
	for _, payload := range []string{`{"type":"text","value":"An essay."}`, `{"type":"text","value":""}`, `nonsense`} {
		got := domain.Grading.Grade(domain.GradableQuestion{Type: "essay", Points: 9}, []byte(payload))
		if !got.RequiresManual || got.Score != 0 {
			t.Errorf("%s: %+v, want it left for the teacher", payload, got)
		}
	}
}

func TestTrueFalseAcceptsBothShapes(t *testing.T) {
	gradeAll(t, domain.GradableQuestion{Type: "true_false", Points: 1, Options: choice(false, true)}, []gradeCase{
		{"the option the engine sends", `{"type":"choice","optionIds":["b"]}`, 1},
		{"the wrong option", `{"type":"choice","optionIds":["a"]}`, 0},
		{"a value", `{"type":"true_false","value":false}`, 1},
		{"the wrong value", `{"type":"true_false","value":true}`, 0},
	})
}
