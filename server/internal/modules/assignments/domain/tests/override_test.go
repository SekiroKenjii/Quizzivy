package domain_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"quizzivy/internal/modules/assignments/domain"
)

func count(n int) *int { return &n }

func TestAnOverrideRequestIsRefusedWhenItSaysNothingOrContradictsItself(t *testing.T) {
	now := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	ahead := now.Add(time.Hour)
	cases := []struct {
		name   string
		in     domain.OverrideInput
		fields []string
	}{
		{"a later close", domain.OverrideInput{ClosesAt: &ahead, Reason: "ốm"}, nil},
		{"minutes added to the close", domain.OverrideInput{ExtendBy: count(30), Reason: "ốm"}, nil},
		{"a longer time limit", domain.OverrideInput{DurationMin: count(90), Reason: "ốm"}, nil},
		{"more attempts", domain.OverrideInput{ExtraAttempts: count(1), Reason: "ốm"}, nil},
		{"no attempts is still a choice", domain.OverrideInput{ExtraAttempts: count(0), Reason: "ốm"}, nil},
		{"nothing to change", domain.OverrideInput{Reason: "ốm"}, []string{"extendBy"}},
		{"a reason of only whitespace", domain.OverrideInput{ExtendBy: count(30), Reason: " \t\n "}, []string{"reason"}},
		{"both ways to name the close", domain.OverrideInput{ExtendBy: count(30), ClosesAt: &ahead, Reason: "ốm"}, []string{"closesAt"}},
		{"nothing to change and no reason", domain.OverrideInput{}, []string{"extendBy", "reason"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.in.Now = now
			err := c.in.Validate()
			var invalid *domain.ValidationError
			var got []string
			if errors.As(err, &invalid) {
				for _, f := range invalid.Fields {
					got = append(got, f.Field)
				}
			} else if err != nil {
				t.Fatalf("Validate = %v, want a ValidationError or nil", err)
			}
			slices.Sort(got)
			if !slices.Equal(got, c.fields) {
				t.Errorf("refused fields = %v, want %v", got, c.fields)
			}
		})
	}
}

func TestTheStoredReasonIsTrimmed(t *testing.T) {
	in := domain.OverrideInput{Reason: "  gia hạn cho em\n"}
	if got := in.CleanReason(); got != "gia hạn cho em" {
		t.Errorf("CleanReason = %q", got)
	}
}
