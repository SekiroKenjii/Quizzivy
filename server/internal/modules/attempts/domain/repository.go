package domain

import (
	"context"
	"time"
)

// Repository persists attempts: their creation and resumption, the answers
// and events saved while they run, and the teacher's interventions.
type Repository interface {
	Rules(ctx context.Context, assignmentID, studentID string) (Rules, error)
	RulesFor(ctx context.Context, assignmentID string) (Rules, error)
	Live(ctx context.Context, assignmentID, studentID string) (AttemptRecord, error)
	Tally(ctx context.Context, assignmentID, studentID string) (Tally, error)
	Create(ctx context.Context, in CreateInput) (AttemptRecord, error)
	Resume(ctx context.Context, in ResumeInput) (AttemptRecord, bool, error)
	ByID(ctx context.Context, attemptID, studentID string) (AttemptRecord, error)
	Rebeacon(ctx context.Context, attemptID string, hash []byte) error
	Sections(ctx context.Context, testVersionID string) ([]Section, error)
	Questions(ctx context.Context, testVersionID string) ([]Question, error)
	Answers(ctx context.Context, attemptID string) (map[string][]byte, error)
	AudioPlays(ctx context.Context, attemptID string) (map[string]int, error)
	RecordPlay(ctx context.Context, attemptID, studentID, questionID string, now time.Time) (Plays, error)
	Save(ctx context.Context, in SaveInput, now time.Time) (SaveResult, error)
	Flush(ctx context.Context, in FlushInput, now time.Time) error
	Submit(ctx context.Context, attemptID, studentID string, reason Reason, now time.Time) (AttemptRecord, error)
	ExpireIfDue(ctx context.Context, attemptID string, now time.Time) error
	DueAttempts(ctx context.Context, assignmentID string, now time.Time) ([]string, error)
	LoadResult(ctx context.Context, a AttemptRecord) (Result, error)
	Monitor(ctx context.Context, assignmentID string, now time.Time) (Monitor, error)
	Extend(ctx context.Context, req Request, attemptID string, minutes int, reason string, now time.Time) (Attempt, error)
	Void(ctx context.Context, req Request, attemptID, reason string, now time.Time) (Attempt, error)
	Reset(ctx context.Context, req Request, attemptID, reason string, now time.Time) (Attempt, error)
	Flag(ctx context.Context, req Request, attemptID string, flagged bool, reason string, now time.Time) (Attempt, error)
}

// ReviewRepository is the teacher's side of a paper: reading it with the
// grading key, marking it, and noting it.
type ReviewRepository interface {
	Get(ctx context.Context, attemptID string) (Review, error)
	Grade(ctx context.Context, attemptID, graderID string, items []GradeItem) (Score, error)
	Finish(ctx context.Context, attemptID string) (Attempt, error)
	SetNote(ctx context.Context, attemptID string, note *string) error
	AnswersForQuestion(ctx context.Context, assignmentID, questionID string) (ByQuestion, error)
}

// TimelineRepository reads the integrity events an attempt recorded.
type TimelineRepository interface {
	Timeline(ctx context.Context, attemptID string) (Timeline, error)
}
