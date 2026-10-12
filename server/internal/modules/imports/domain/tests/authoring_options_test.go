package domain_test

import (
	"errors"
	"github.com/google/uuid"
	"quizzivy/internal/modules/imports/domain"
	"testing"
)

func TestIncludedImportCapDoesNotDependOnAnswerKnownAndExcludedIsIgnored(t *testing.T) {
	for _, state := range []domain.AnswerState{domain.AnswerKnown, domain.AnswerUnknown, domain.AnswerConflict} {
		for _, n := range []int{8, 9} {
			q := choice("cap", "A")
			q.Answer.State = state
			for len(q.Options) < n {
				q.Options = append(q.Options, domain.DraftOption{ID: uuid.NewString(), Label: "extra", Content: prose("extra")})
			}
			d := draft(questionItem(q))
			r := domain.Assess(d)
			capCount := 0
			for _, f := range r.Findings {
				if f.Code == domain.CodeInvalidQuestion && f.Field == "options" {
					capCount++
				}
			}
			if capCount != map[bool]int{true: 1, false: 0}[n > 8] {
				t.Fatalf("%s/%d cap findings %+v", state, n, r.Findings)
			}
			if n > 8 {
				if _, err := domain.Plan(d, "fallback", uuid.NewString); !errors.Is(err, domain.ErrNotReady) {
					t.Fatalf("oversized plan: %v", err)
				}
			}
			q.Excluded = &domain.Exclusion{Reason: "skip"}
			r = domain.Assess(draft(questionItem(q), questionItem(choice("good", "A"))))
			if !r.Ready || r.Summary.Included != 1 || r.Summary.Excluded != 1 || r.Summary.TotalPoints != "1.00" {
				t.Fatalf("excluded affected review: %+v", r)
			}
		}
	}
}
