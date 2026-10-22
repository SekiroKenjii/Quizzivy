package domain

import (
	"encoding/json"
	"math"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// Grade scores one answer.
func (GradingManager) Grade(q GradableQuestion, payload []byte) GradeResult {
	if q.Type == "short_answer" || q.Type == Essay {
		return GradeResult{RequiresManual: true}
	}
	if len(payload) == 0 {
		return GradeResult{}
	}

	var correct bool
	switch q.Type {
	case "multiple_choice":
		if q.SelectCount != nil {
			return GradeResult{Score: gradeChooseN(q, *q.SelectCount, payload)}
		}
		correct = gradeChoice(q, payload)
	case "single_choice", "tfng":
		correct = gradeChoice(q, payload)
	case "matching":
		return GradeResult{Score: gradeMatching(q, payload)}
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
	ID          string
	Type        string
	Points      float64
	SelectCount *int
	Options     []GradableOption
	Blanks      []GradableBlank
	MatchItems  []GradableMatchItem
}

// GradableMatchItem is a matching item and the option that answers it.
type GradableMatchItem struct {
	ID       string
	OptionID string
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

func gradeChooseN(q GradableQuestion, n int, payload []byte) float64 {
	var answer struct {
		OptionIDs []string `json:"optionIds"`
	}
	if json.Unmarshal(payload, &answer) != nil || n <= 0 {
		return 0
	}
	chosen := make(map[string]bool, len(answer.OptionIDs))
	for _, id := range answer.OptionIDs {
		chosen[id] = true
	}
	if len(chosen) > n {
		return 0
	}
	right := 0
	for _, o := range q.Options {
		if o.Correct && chosen[o.ID] {
			right++
		}
	}
	return share(q.Points, right, n)
}

func gradeMatching(q GradableQuestion, payload []byte) float64 {
	var answer struct {
		Pairs map[string]string `json:"pairs"`
	}
	if json.Unmarshal(payload, &answer) != nil || len(q.MatchItems) == 0 {
		return 0
	}
	right := 0
	for _, item := range q.MatchItems {
		if picked, ok := answer.Pairs[item.ID]; ok && picked == item.OptionID {
			right++
		}
	}
	return share(q.Points, right, len(q.MatchItems))
}

func share(points float64, right, of int) float64 {
	if right == 0 {
		return 0
	}
	return math.Round(points*float64(right)/float64(of)*100) / 100
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
	return share(q.Points, matched, len(q.Blanks))
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
