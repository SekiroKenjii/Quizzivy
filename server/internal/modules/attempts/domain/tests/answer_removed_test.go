package domain_test

import (
	"testing"

	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/retention"
)

func TestOnlyTheRetentionMarkerReadsAsARemovedAnswer(t *testing.T) {
	for payload, want := range map[string]bool{
		retention.RemovedAnswer:                  true,
		`{"removed":true}`:                       true,
		`{"removed": false}`:                     false,
		`{"type": "text", "removed": true}`:      false,
		`{"type": "text", "text": "removed"}`:    false,
		`{"type": "choice", "optionIds": ["a"]}`: false,
		`null`:                                   false,
		``:                                       false,
	} {
		if got := domain.AnswerRemoved([]byte(payload)); got != want {
			t.Errorf("AnswerRemoved(%q) = %v, want %v", payload, got, want)
		}
	}
}
