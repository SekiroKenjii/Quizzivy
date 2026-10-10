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

// ClassScore is how a class did: the sum of the points earned and of the
// points on offer in the best graded attempt of each live member on each
// assignment that targets the class, and the manual answers still unmarked in
// those attempts.
type ClassScore struct {
	Earned        float64
	Total         float64
	PendingManual int
}

// ClassSource answers ClassScore for a set of classes in one query. The caller
// has already scoped the ids, so the source applies no scope of its own. A
// class with nothing graded has no entry. The attempts module provides it.
type ClassSource interface {
	ClassScores(ctx context.Context, classIDs []string) (map[string]ClassScore, error)
}
