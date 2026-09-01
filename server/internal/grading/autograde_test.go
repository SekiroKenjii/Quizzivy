package grading_test

import (
	"testing"

	"quizzivy/internal/grading"
)

func choice(correct ...bool) []grading.Option {
	out := make([]grading.Option, len(correct))
	for i, c := range correct {
		out[i] = grading.Option{ID: string(rune('a' + i)), Ordinal: i, Correct: c}
	}
	return out
}

func TestSingleChoice(t *testing.T) {
	q := grading.Question{Type: "single_choice", Points: 5, Options: choice(true, false, false)}

	for _, c := range []struct {
		name    string
		payload string
		want    float64
	}{
		{"the correct option", `{"type":"choice","optionIds":["a"]}`, 5},
		{"a wrong option", `{"type":"choice","optionIds":["b"]}`, 0},
		{"nothing selected", `{"type":"choice","optionIds":[]}`, 0},
		{"the right one and a wrong one", `{"type":"choice","optionIds":["a","b"]}`, 0},
		{"an option from another question", `{"type":"choice","optionIds":["a","zz"]}`, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := grading.Grade(q, []byte(c.payload)).Score; got != c.want {
				t.Errorf("score %v, want %v", got, c.want)
			}
		})
	}
}

// [O-09] All-or-nothing. Every correct option and no incorrect one, or zero.
func TestMultipleChoiceIsAllOrNothing(t *testing.T) {
	q := grading.Question{Type: "multiple_choice", Points: 4, Options: choice(true, true, false, false)}

	for _, c := range []struct {
		name    string
		payload string
		want    float64
	}{
		{"both correct", `{"type":"choice","optionIds":["a","b"]}`, 4},
		{"both correct, other order", `{"type":"choice","optionIds":["b","a"]}`, 4},
		{"one of two correct", `{"type":"choice","optionIds":["a"]}`, 0},
		{"both correct plus a wrong one", `{"type":"choice","optionIds":["a","b","c"]}`, 0},
		{"everything selected", `{"type":"choice","optionIds":["a","b","c","d"]}`, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := grading.Grade(q, []byte(c.payload)).Score; got != c.want {
				t.Errorf("score %v, want %v", got, c.want)
			}
		})
	}
}

// The key lives in the option list, and the boolean maps to an ordinal: the
// editor writes True at 0 and False at 1 and gives the teacher no way to rename
// either. Matching on the text would be matching on authored English in a
// Vietnamese product.
func TestTrueFalseReadsTheKeyByOrdinalNotByText(t *testing.T) {
	trueIsCorrect := grading.Question{Type: "true_false", Points: 2, Options: choice(true, false)}
	falseIsCorrect := grading.Question{Type: "true_false", Points: 2, Options: choice(false, true)}

	for _, c := range []struct {
		name    string
		q       grading.Question
		payload string
		want    float64
	}{
		{"answering true when true is correct", trueIsCorrect, `{"type":"true_false","value":true}`, 2},
		{"answering false when true is correct", trueIsCorrect, `{"type":"true_false","value":false}`, 0},
		{"answering false when false is correct", falseIsCorrect, `{"type":"true_false","value":false}`, 2},
		{"answering true when false is correct", falseIsCorrect, `{"type":"true_false","value":true}`, 0},
		{"no value at all", trueIsCorrect, `{"type":"true_false"}`, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := grading.Grade(c.q, []byte(c.payload)).Score; got != c.want {
				t.Errorf("score %v, want %v", got, c.want)
			}
		})
	}
}

func TestFillBlankForgivesTypingAndNotTheAnswer(t *testing.T) {
	q := grading.Question{
		Type: "fill_blank", Points: 3,
		Blanks: []grading.Blank{{ID: "b1", Accepted: []string{"has lived", "has been living"}}},
	}

	for _, c := range []struct {
		name  string
		given string
		want  float64
	}{
		{"exactly", "has lived", 3},
		{"a trailing space", "has lived ", 3},
		{"a leading space", "  has lived", 3},
		{"two spaces in the middle", "has  lived", 3},
		{"different case", "Has Lived", 3},
		{"the second accepted answer", "has been living", 3},
		{"a wrong answer", "lived", 0},
		{"empty", "", 0},
		{"only whitespace", "   ", 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			payload := `{"type":"fill_blank","values":{"b1":` + quote(c.given) + `}}`
			if got := grading.Grade(q, []byte(payload)).Score; got != c.want {
				t.Errorf("score %v, want %v for %q", got, c.want, c.given)
			}
		})
	}
}

// Diacritics are meaning, not formatting. This is an English test written for
// Vietnamese students, and folding accents would make a whole class of answer
// unmarkable -- "ha noi" is not "Hà Nội" the way "hanoi " is "Hanoi".
func TestFillBlankKeepsDiacritics(t *testing.T) {
	q := grading.Question{
		Type: "fill_blank", Points: 3,
		Blanks: []grading.Blank{{ID: "b1", Accepted: []string{"Hà Nội"}}},
	}

	for _, c := range []struct {
		name  string
		given string
		want  float64
	}{
		{"exactly, with a trailing space", "Hà Nội ", 3},
		{"lower case, diacritics intact", "hà nội", 3},
		{"diacritics stripped", "Ha Noi", 0},
		{"partially stripped", "Hà Noi", 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			payload := `{"type":"fill_blank","values":{"b1":` + quote(c.given) + `}}`
			if got := grading.Grade(q, []byte(payload)).Score; got != c.want {
				t.Errorf("score %v, want %v for %q", got, c.want, c.given)
			}
		})
	}
}

func TestACaseSensitiveBlankSaysSo(t *testing.T) {
	q := grading.Question{
		Type: "fill_blank", Points: 3,
		Blanks: []grading.Blank{{ID: "b1", CaseSensitive: true, Accepted: []string{"London"}}},
	}
	if got := grading.Grade(q, []byte(`{"type":"fill_blank","values":{"b1":"London "}}`)).Score; got != 3 {
		t.Errorf("score %v, want 3 — whitespace is forgiven even when case is not", got)
	}
	if got := grading.Grade(q, []byte(`{"type":"fill_blank","values":{"b1":"london"}}`)).Score; got != 0 {
		t.Errorf("score %v, want 0 — the teacher asked for case to matter", got)
	}
}

// [O-17] Every blank or nothing, following O-09's rule for the other multi-part
// type. Change this and the open item together.
func TestAPartlyCorrectFillBlankScoresZero(t *testing.T) {
	q := grading.Question{
		Type: "fill_blank", Points: 6,
		Blanks: []grading.Blank{
			{ID: "b1", Accepted: []string{"has lived"}},
			{ID: "b2", Accepted: []string{"2019"}},
		},
	}
	both := `{"type":"fill_blank","values":{"b1":"has lived","b2":"2019"}}`
	if got := grading.Grade(q, []byte(both)).Score; got != 6 {
		t.Errorf("both blanks right scored %v, want 6", got)
	}
	one := `{"type":"fill_blank","values":{"b1":"has lived","b2":"2020"}}`
	if got := grading.Grade(q, []byte(one)).Score; got != 0 {
		t.Errorf("one of two blanks right scored %v, want 0", got)
	}
	missing := `{"type":"fill_blank","values":{"b1":"has lived"}}`
	if got := grading.Grade(q, []byte(missing)).Score; got != 0 {
		t.Errorf("a blank left empty scored %v, want 0", got)
	}
}

// [D-19] Not zero-because-wrong. Nothing has been decided yet, and this is what
// §7's pendingManual counts.
func TestShortAnswerIsLeftForTheTeacher(t *testing.T) {
	q := grading.Question{Type: "short_answer", Points: 5}
	got := grading.Grade(q, []byte(`{"type":"text","value":"Tôi dậy lúc 6 giờ."}`))
	if !got.RequiresManual {
		t.Error("short_answer must be marked for manual grading")
	}
	if got.Score != 0 {
		t.Errorf("score %v, want 0 until a teacher says otherwise", got.Score)
	}
}

// A student who never answered and one whose answer did not survive are the
// same zero, and there is nobody to hand an error to at grading time.
func TestAnUnanswerableAnswerScoresZeroRatherThanFailing(t *testing.T) {
	q := grading.Question{Type: "single_choice", Points: 5, Options: choice(true, false)}
	for _, payload := range []string{"", "not json at all", `{"type":"choice"}`, `null`} {
		if got := grading.Grade(q, []byte(payload)).Score; got != 0 {
			t.Errorf("payload %q scored %v, want 0", payload, got)
		}
	}
}

func quote(s string) string {
	out := []rune{'"'}
	for _, r := range s {
		if r == '"' || r == '\\' {
			out = append(out, '\\')
		}
		out = append(out, r)
	}
	return string(append(out, '"'))
}
