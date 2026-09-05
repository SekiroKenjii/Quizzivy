package domain

import (
	"encoding/json"
	"math"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// GradableQuestion is everything scoring needs and nothing it does not. No prompt, no
// media, no explanation: a bug here should be unable to reach a student's
// screen, and it cannot leak what it never holds.
type GradableQuestion struct {
	ID      string
	Type    string
	Points  float64
	Options []GradableOption
	Blanks  []GradableBlank
}

// GradableOption carries the key. Ordinal matters for true_false and only there.
type GradableOption struct {
	ID      string
	Ordinal int
	Correct bool
}

type GradableBlank struct {
	ID            string
	CaseSensitive bool
	Accepted      []string
}

// GradeResult is one graded answer. Score is zero when RequiresManual, because
// nothing has been decided yet -- not because the student got it wrong.
type GradeResult struct {
	Score          float64
	RequiresManual bool
}

// Grade scores one answer.
//
// An unparseable or absent payload scores zero rather than erroring. A student
// who never answered and a student whose answer did not survive are the same
// zero, and there is nobody to hand an error to at grading time.
func Grade(q GradableQuestion, payload []byte) GradeResult {
	if q.Type == "short_answer" {
		// [D-19] Not zero-because-wrong.
		return GradeResult{RequiresManual: true}
	}
	if len(payload) == 0 {
		return GradeResult{}
	}

	var correct bool
	switch q.Type {
	case "single_choice", "multiple_choice":
		correct = gradeChoice(q, payload)
	case "true_false":
		correct = gradeTrueFalse(q, payload)
	case "fill_blank":
		// The one type that is not all-or-nothing. See gradeFillBlank.
		return GradeResult{Score: gradeFillBlank(q, payload)}
	default:
		return GradeResult{}
	}
	if !correct {
		return GradeResult{}
	}
	return GradeResult{Score: q.Points}
}

// gradeChoice is all-or-nothing: every correct option selected and no incorrect
// one (O-09).
//
// §7 gives multiple_choice a points value and an isCorrect flag per option but
// never states the rule. Partial credit is a real pedagogical choice that
// changes this code, the result display, and what "correct" means in review, so
// it is a decision to take deliberately rather than to fall into.
func gradeChoice(q GradableQuestion, payload []byte) bool {
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
	// Everything chosen has to be an option of THIS question.
	return recognised == len(chosen)
}

// gradeTrueFalse compares against the option list, because that is where the
// key lives.
func gradeTrueFalse(q GradableQuestion, payload []byte) bool {
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

// gradeFillBlank awards each blank its share (O-17).
func gradeFillBlank(q GradableQuestion, payload []byte) float64 {
	var answer struct {
		Values map[string]string `json:"values"`
	}
	if json.Unmarshal(payload, &answer) != nil {
		return 0
	}
	if len(q.Blanks) == 0 {
		return 0
	}

	matched := 0
	for _, blank := range q.Blanks {
		if matches(blank, answer.Values[blank.ID]) {
			matched++
		}
	}
	if matched == 0 {
		return 0
	}
	return math.Round(q.Points*float64(matched)/float64(len(q.Blanks))*100) / 100
}

func matches(blank GradableBlank, given string) bool {
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
func normalise(s string, caseSensitive bool) string {
	out := strings.Join(strings.Fields(norm.NFC.String(s)), " ")
	if caseSensitive {
		return out
	}
	return strings.ToLower(out)
}
