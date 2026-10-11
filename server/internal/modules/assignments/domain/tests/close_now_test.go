package domain_test

import (
	"errors"
	"testing"
	"time"

	"quizzivy/internal/modules/assignments/domain"
)

func TestCloseNowNeedsAWindowThatHasOpened(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	valid := func(opens time.Time, closeNow bool) domain.WriteInput {
		return domain.WriteInput{
			ClassIDs: []string{"class"}, OpensAt: opens, ClosesAt: now.Add(48 * time.Hour),
			Review: domain.Review{Release: domain.ReleaseOnSubmit}, CloseNow: closeNow, Now: now,
		}
	}

	for _, tc := range []struct {
		name    string
		in      domain.WriteInput
		refused bool
	}{
		{"closing now with the opening a nanosecond ahead", valid(now.Add(time.Nanosecond), true), true},
		{"closing now with the opening an hour ahead", valid(now.Add(time.Hour), true), true},
		{"closing now at the instant the window opens", valid(now, true), false},
		{"closing now after the window opened", valid(now.Add(-time.Hour), true), false},
		{"saving a window that opens ahead without closing", valid(now.Add(time.Hour), false), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.in.Validate()
			var invalid *domain.ValidationError
			named := errors.As(err, &invalid) && len(invalid.Fields) == 1 && invalid.Fields[0].Field == "closeNow"
			if tc.refused != named || (!tc.refused && err != nil) {
				t.Errorf("Validate = %v, want refused=%v naming closeNow only", err, tc.refused)
			}
		})
	}
}
