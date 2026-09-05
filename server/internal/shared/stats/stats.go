package stats

import (
	"context"
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

// Source answers the figures for a set of students in one query; the attempts module provides it.
type Source interface {
	StudentStats(ctx context.Context, ids []string) (map[string]Student, error)
}
