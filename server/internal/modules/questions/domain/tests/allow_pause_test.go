package domain_test

import (
	"testing"

	"quizzivy/internal/modules/questions/domain"
)

func TestAnAudioPolicyThatDoesNotSayAllowsPausing(t *testing.T) {
	off, on := false, true
	for name, tc := range map[string]struct {
		policy domain.AudioPolicy
		want   bool
	}{
		"unset": {domain.AudioPolicy{}, true},
		"on":    {domain.AudioPolicy{AllowPause: &on}, true},
		"off":   {domain.AudioPolicy{AllowPause: &off}, false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := tc.policy.PauseAllowed(); got != tc.want {
				t.Fatalf("PauseAllowed() = %v, want %v", got, tc.want)
			}
		})
	}
}
