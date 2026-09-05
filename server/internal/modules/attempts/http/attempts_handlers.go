package http

import (
	"context"
	"log/slog"
	"time"

	"quizzivy/internal/modules/attempts/domain"
	identitydomain "quizzivy/internal/modules/identity/domain"
	mediaapp "quizzivy/internal/modules/media/application"
	mediadomain "quizzivy/internal/modules/media/domain"
)

// Service is the student's side of an attempt plus the teacher's interventions.
type Service interface {
	StartOrResume(ctx context.Context, assignmentID, studentID string) (domain.Session, error)
	Get(ctx context.Context, attemptID, studentID string) (domain.Session, error)
	Save(ctx context.Context, in domain.SaveInput) (domain.SaveResult, error)
	RecordPlay(ctx context.Context, attemptID, studentID, questionID string) (domain.Plays, error)
	Flush(ctx context.Context, in domain.FlushInput) error
	Submit(ctx context.Context, attemptID, studentID string, reason domain.Reason) (domain.Attempt, error)
	Result(ctx context.Context, attemptID, studentID string) (domain.Result, error)
	Monitor(ctx context.Context, assignmentID string) (domain.Monitor, error)
	Extend(ctx context.Context, req domain.Request, attemptID string, minutes int, reason string) (domain.Attempt, error)
	Flag(ctx context.Context, req domain.Request, attemptID string, flagged bool, reason string) (domain.Attempt, error)
	Reset(ctx context.Context, req domain.Request, attemptID, reason string) (domain.Attempt, error)
	Void(ctx context.Context, req domain.Request, attemptID, reason string) (domain.Attempt, error)
}

// Review is the teacher's reading and marking of one paper.
type Review interface {
	Get(ctx context.Context, attemptID string) (domain.Review, error)
	Grade(ctx context.Context, attemptID, graderID string, items []domain.GradeItem) (domain.Score, error)
	Finish(ctx context.Context, attemptID string) (domain.Attempt, error)
	SetNote(ctx context.Context, attemptID string, note *string) error
	AnswersForQuestion(ctx context.Context, assignmentID, questionID string) (domain.ByQuestion, error)
}

type Integrity interface {
	Timeline(ctx context.Context, attemptID string) (domain.Timeline, error)
}

// Media resolves a question's audio; nil when object storage is off.
type Media interface {
	Get(ctx context.Context, id string) (mediadomain.Asset, error)
	SignedURL(ctx context.Context, asset mediadomain.Asset) (string, error)
	MintForStudent(ctx context.Context, studentID, assetID string) (mediaapp.SignedURLResult, error)
	SignedURLTTL() time.Duration
}

// Students names the student behind a paper for the teacher's review.
type Students interface {
	Get(ctx context.Context, id string) (identitydomain.Student, error)
}

type Attempts struct {
	attempts  Service
	review    Review
	integrity Integrity
	media     Media
	students  Students
	logger    *slog.Logger
}

func NewAttempts(attempts Service, review Review, integrity Integrity, media Media, students Students, logger *slog.Logger) Attempts {
	return Attempts{attempts: attempts, review: review, integrity: integrity, media: media, students: students, logger: logger}
}

func (h Attempts) log() *slog.Logger {
	if h.logger == nil {
		return slog.New(slog.DiscardHandler)
	}
	return h.logger
}
