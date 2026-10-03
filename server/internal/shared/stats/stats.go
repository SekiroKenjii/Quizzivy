package stats

import (
	"context"
	"quizzivy/internal/shared/access"
	"time"
)

// Student is the teaching figures every roster shows beside a student.
type Student struct {
	SubmittedCount int
	ScoreEarned    *float64
	ScoreTotal     *float64
	PendingManual  int
	FlaggedCount   int
	LiveAttempt    bool
	LastAttemptAt  *time.Time
}

// Source answers the figures for a set of students in one query, over only
// the assignments the scope reaches (visibility.AssignmentIDs), or every one
// under scope.all; a zero scope reaches none. The attempts module provides it.
type Source interface {
	StudentStats(ctx context.Context, scope access.Scope, ids []string) (map[string]Student, error)
}
