package domain_test

import (
	"errors"
	"quizzivy/internal/modules/questions/domain"
	"testing"
)

func authoredChoice(n int) domain.Input {
	in := domain.Input{Type: domain.SingleChoice, Prompt: "Pick", Points: "1"}
	for i := range n {
		in.Options = append(in.Options, domain.OptionInput{Text: "option", IsCorrect: i == 0})
	}
	return in
}

func TestAuthoringEightAndStoredNineHaveSeparateBoundaries(t *testing.T) {
	for _, n := range []int{2, 8, 9, 10} {
		in := authoredChoice(n)
		if err := in.Validate(nil); err != nil {
			t.Fatalf("stored %d invalid: %v", n, err)
		}
		err := in.ValidateAuthoring()
		if (err != nil) != (n > 8) {
			t.Fatalf("authoring %d: %v", n, err)
		}
		if n > 8 {
			var invalid *domain.ValidationError
			if !errors.As(err, &invalid) || len(invalid.Fields) != 1 || invalid.Fields[0].Field != "options" {
				t.Fatalf("cap fields: %v", err)
			}
		}
	}
	in := domain.Input{Type: domain.ShortAnswer, Options: make([]domain.OptionInput, 9)}
	if err := in.ValidateAuthoring(); err != nil {
		t.Fatalf("cap applied to non-choice: %v", err)
	}
}

func TestMetadataKnownValuesAndNullAreValidButUnknownIsNot(t *testing.T) {
	in := authoredChoice(2)
	for _, value := range []domain.Level{"pre_a1", "a1", "a2", "b1", "b2", "c1", "c2"} {
		in.Level = &value
		if err := in.Validate(nil); err != nil {
			t.Fatal(err)
		}
	}
	in.Level = nil
	for _, value := range []domain.Skill{"grammar", "vocabulary", "reading", "listening", "writing", "speaking"} {
		in.Skill = &value
		if err := in.Validate(nil); err != nil {
			t.Fatal(err)
		}
	}
	in.Skill = nil
	for _, value := range []string{"", "A1", "unknown"} {
		level, skill := domain.Level(value), domain.Skill(value)
		in.Level, in.Skill = &level, nil
		if err := in.Validate(nil); err == nil {
			t.Fatalf("invalid level %q accepted", value)
		}
		in.Level, in.Skill = nil, &skill
		if err := in.Validate(nil); err == nil {
			t.Fatalf("invalid skill %q accepted", value)
		}
	}
	in.Skill = nil
	if err := in.Validate(nil); err != nil {
		t.Fatalf("unset metadata: %v", err)
	}
}
