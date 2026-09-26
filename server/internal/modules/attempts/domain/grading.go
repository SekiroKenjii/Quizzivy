package domain

import (
	"encoding/json"
	"math"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// Grade scores one answer.
func (GradingManager) Grade(q GradableQuestion, payload []byte) GradeResult {
	if q.Type == "short_answer" {

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

		return GradeResult{Score: gradeFillBlank(q, payload)}
	default:
		return GradeResult{}
	}
	if !correct {
		return GradeResult{}
	}
	return GradeResult{Score: q.Points}
}

// GradingManager scores an answer against the key the version froze.
type GradingManager struct{}

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

// GradableOption carries the key. Ordinal matters only for a true_false answer
// stored as a boolean.
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

	return recognised == len(chosen)
}

func gradeTrueFalse(q GradableQuestion, payload []byte) bool {
	var answer struct {
		OptionIDs []string `json:"optionIds"`
		Value     *bool    `json:"value"`
	}
	if json.Unmarshal(payload, &answer) != nil {
		return false
	}
	if answer.OptionIDs != nil {
		return len(answer.OptionIDs) == 1 && gradeChoice(q, payload)
	}
	if answer.Value == nil {
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

func normalise(s string, caseSensitive bool) string {
	out := strings.Join(strings.Fields(norm.NFC.String(s)), " ")
	if caseSensitive {
		return out
	}
	return strings.ToLower(out)
}
