package domain

import (
	"strings"
)

// GradeItem is one manual mark: the question, the points, and a comment the
// student will read.
type GradeItem struct {
	QuestionID string
	Points     float64
	Comment    *string
}

// GradeItemError names an item that could not be marked and why -- `not_on_paper`,
// `unanswered` or `above_ceiling`.
type GradeItemError struct {
	QuestionID string
	Reason     string
}

type GradeValidationError struct{ Items []GradeItemError }

func (e *GradeValidationError) Error() string {
	parts := make([]string, len(e.Items))
	for i, it := range e.Items {
		parts[i] = it.QuestionID + ": " + it.Reason
	}
	return "review: " + strings.Join(parts, "; ")
}
