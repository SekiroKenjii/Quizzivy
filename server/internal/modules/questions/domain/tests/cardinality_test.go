package domain_test

import (
	"quizzivy/internal/modules/questions/domain"
	"testing"
)

func TestTrueFalseRequiresExactlyOneKey(t *testing.T) {
	for _, correct := range []int{0, 1, 2} {
		in := choice(domain.OptionInput{Text: "Đúng", IsCorrect: correct > 0}, domain.OptionInput{Text: "Sai", IsCorrect: correct > 1})
		in.Type = domain.TrueFalse
		err := in.Validate(nil)
		if (err == nil) != (correct == 1) {
			t.Fatalf("%d keys: %v", correct, err)
		}
	}
}

func TestInternalQuestionWritesRequireExactPositivePoints(t *testing.T) {
	for _, points := range []string{"", "NaN", "Inf", "1.001", "1000000", "-1", "0", "1e2"} {
		in := domain.Input{Type: domain.ShortAnswer, Prompt: "Câu hỏi", Points: points}
		if !hasField(t, in.Validate(nil), "points") {
			t.Errorf("accepted invalid points %q", points)
		}
	}
	for text, want := range map[string]int64{"0.01": 1, "0.29": 29, "10": 1000, "1.5": 150, "999999.99": 99_999_999} {
		if units, ok := domain.PointUnits(text); !ok || units != want {
			t.Errorf("%s: %d/%v, want %d", text, units, ok, want)
		}
	}
}
