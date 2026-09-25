package domain_test

import (
	"encoding/json"
	"quizzivy/internal/modules/tests/domain"
	"testing"
)

func TestPublishingRevalidatesRichOptionProjection(t *testing.T) {
	q := base()
	q.Type = "single_choice"
	q.Options = []domain.DraftOption{
		{Text: "think", IsCorrect: true, Content: json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"other","marks":["underline"]}]}]}`)},
		{Text: "other"},
	}
	violation := onlyViolation(t, draftWith(q))
	if violation.Rule != domain.OptionContentValid || violation.QuestionID != q.SourceID {
		t.Fatalf("invalid content not anchored to the question: %+v", violation)
	}
}
