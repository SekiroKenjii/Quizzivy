// Package grading scores one answer against one question.
//
// Pure: no database, no clock, no attempt. That is what makes the rules -- which
// are the part a teacher will argue with -- testable one case at a time, and it
// is why this is its own package rather than a method on the attempt.
package grading

import (
	"encoding/json"
	"strings"
)

// Question is everything scoring needs and nothing it does not. No prompt, no
// media, no explanation: a bug here should be unable to reach a student's
// screen, and it cannot leak what it never holds.
type Question struct {
	ID      string
	Type    string
	Points  float64
	Options []Option
	Blanks  []Blank
}

// Option carries the key. Ordinal matters for true_false and only there.
type Option struct {
	ID      string
	Ordinal int
	Correct bool
}

type Blank struct {
	ID            string
	CaseSensitive bool
	Accepted      []string
}

// Result is one graded answer. Score is zero when RequiresManual, because
// nothing has been decided yet -- not because the student got it wrong.
type Result struct {
	Score          float64
	RequiresManual bool
}

// Grade scores one answer.
//
// An unparseable or absent payload scores zero rather than erroring. A student
// who never answered and a student whose answer did not survive are the same
// zero, and there is nobody to hand an error to at grading time.
func Grade(q Question, payload []byte) Result {
	if q.Type == "short_answer" {
		// [D-19] Not zero-because-wrong. The teacher decides, and until they do
		// this is what §7's pendingManual counts.
		return Result{RequiresManual: true}
	}
	if len(payload) == 0 {
		return Result{}
	}

	var correct bool
	switch q.Type {
	case "single_choice", "multiple_choice":
		correct = gradeChoice(q, payload)
	case "true_false":
		correct = gradeTrueFalse(q, payload)
	case "fill_blank":
		correct = gradeFillBlank(q, payload)
	default:
		return Result{}
	}
	if !correct {
		return Result{}
	}
	return Result{Score: q.Points}
}

// gradeChoice is all-or-nothing: every correct option selected and no incorrect
// one (O-09).
//
// §7 gives multiple_choice a points value and an isCorrect flag per option but
// never states the rule. Partial credit is a real pedagogical choice that
// changes this code, the result display, and what "correct" means in review, so
// it is a decision to take deliberately rather than to fall into.
func gradeChoice(q Question, payload []byte) bool {
	var answer struct {
		OptionIDs []string `json:"optionIds"`
	}
	if json.Unmarshal(payload, &answer) != nil {
		return false
	}

	chosen := make(map[string]bool, len(answer.OptionIDs))
	for _, id := range answer.OptionIDs {
		chosen[id] = true
	}

	recognised := 0
	for _, o := range q.Options {
		if chosen[o.ID] != o.Correct {
			return false
		}
		if chosen[o.ID] {
			recognised++
		}
	}
	// Everything chosen has to be an option of THIS question. The loop above
	// only sees ids it knows, so without this an id from another question rides
	// along unnoticed. Impossible from a correct client, and not a reason to
	// award marks.
	return recognised == len(chosen)
}

// gradeTrueFalse compares against the option list, because that is where the
// key lives.
//
// The two options are fixed in order and never renamed -- the editor writes
// True at ordinal 0 and False at ordinal 1 and gives the teacher no way to
// change either -- so the boolean maps to an ordinal. Matching on the TEXT
// would be worse: it is authored data, it is English in a Vietnamese product,
// and a teacher who found a way to edit it would silently invert the grade.
func gradeTrueFalse(q Question, payload []byte) bool {
	var answer struct {
		Value *bool `json:"value"`
	}
	if json.Unmarshal(payload, &answer) != nil || answer.Value == nil {
		return false
	}

	want := 1
	if *answer.Value {
		want = 0
	}
	for _, o := range q.Options {
		if o.Ordinal == want {
			return o.Correct
		}
	}
	return false
}

// gradeFillBlank requires every blank to match one of its accepted answers.
//
// All-or-nothing, following O-09's rule for the other multi-part type. It is a
// judgement rather than a quotation -- neither §7 nor the open items settle a
// two-blank sentence -- and it is the conservative direction: relaxing to a
// proportion later only ever raises a score, where tightening would lower one
// already reported. See O-17.
func gradeFillBlank(q Question, payload []byte) bool {
	var answer struct {
		Values map[string]string `json:"values"`
	}
	if json.Unmarshal(payload, &answer) != nil {
		return false
	}
	if len(q.Blanks) == 0 {
		return false
	}

	for _, blank := range q.Blanks {
		if !matches(blank, answer.Values[blank.ID]) {
			return false
		}
	}
	return true
}

func matches(blank Blank, given string) bool {
	got := normalise(given, blank.CaseSensitive)
	if got == "" {
		return false
	}
	for _, accepted := range blank.Accepted {
		if got == normalise(accepted, blank.CaseSensitive) {
			return true
		}
	}
	return false
}

// normalise forgives the typing, never the answer.
//
// Surrounding whitespace goes and internal runs collapse, because a trailing
// space from a phone keyboard is not a mistake about English. Case folds unless
// the teacher asked for it.
//
// Diacritics are deliberately NOT folded. This is an English test written for
// Vietnamese students; the Vietnamese that appears in an answer is meaning-
// bearing, and "ha noi" is not "Hà Nội" the way "hanoi " is "Hanoi".
func normalise(s string, caseSensitive bool) string {
	out := strings.Join(strings.Fields(s), " ")
	if caseSensitive {
		return out
	}
	return strings.ToLower(out)
}
