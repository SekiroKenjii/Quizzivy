package domain_test

import (
	"encoding/json"
	"errors"
	"quizzivy/internal/modules/tests/domain"
	"testing"
)

func TestPublicationRevalidatesQuestionProse(t *testing.T) {
	draft := domain.DraftContent{Sections: []domain.DraftSection{{ID: "section", Questions: []domain.DraftQuestion{{SourceID: "question", Type: "short_answer", Points: "1", Prompt: "different", PromptContent: json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"stored","marks":[]}]}]}`)}}}}}
	var err *domain.PublishValidationError
	if !errors.As(domain.Publishing.Validate(draft), &err) {
		t.Fatal("publication accepted mismatched content")
	}
	if len(err.Violations) != 1 || err.Violations[0].Rule != domain.QuestionContentValid || err.Violations[0].QuestionID != "question" {
		t.Fatalf("unanchored error: %+v", err)
	}
}
