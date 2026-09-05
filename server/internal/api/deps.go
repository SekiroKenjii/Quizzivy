package api

import (
	"context"
	"errors"
	identityapp "quizzivy/internal/modules/identity/application"
	identitydomain "quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/paging"
	"time"

	"quizzivy/internal/modules/attempts"
	"quizzivy/internal/modules/integrity"
	mediaapp "quizzivy/internal/modules/media/application"
	mediadomain "quizzivy/internal/modules/media/domain"
	"quizzivy/internal/modules/review"
	"quizzivy/internal/platform/httpx"
)

// DB is the slice of the pool handlers need. An interface rather than the
// concrete pool so tests can substitute one without a live database.
type DB interface {
	Ping(ctx context.Context) error
}

// MediaService is the slice of internal/media the handlers use.
type MediaService interface {
	Upload(ctx context.Context, in mediaapp.UploadInput) (mediadomain.Asset, error)
	SignedURL(ctx context.Context, asset mediadomain.Asset) (string, error)
	List(ctx context.Context, in mediadomain.ListInput) ([]mediadomain.Asset, paging.Page, error)
	TotalBytes(ctx context.Context, kind *mediadomain.Kind) (int64, error)
	Delete(ctx context.Context, in mediadomain.DeleteInput) error
	MintForStudent(ctx context.Context, studentID, assetID string) (mediaapp.SignedURLResult, error)
	// Get resolves one asset, so a question can render its attachment.
	Get(ctx context.Context, id string) (mediadomain.Asset, error)
	SignedURLTTL() time.Duration
}

// AttemptsService is the slice of internal/attempts the handlers use.
type AttemptsService interface {
	StartOrResume(ctx context.Context, assignmentID, studentID string) (attempts.Session, error)
	Get(ctx context.Context, attemptID, studentID string) (attempts.Session, error)
	Save(ctx context.Context, in attempts.SaveInput) (attempts.SaveResult, error)
	RecordPlay(ctx context.Context, attemptID, studentID, questionID string) (attempts.Plays, error)
	Flush(ctx context.Context, in attempts.FlushInput) error
	Submit(ctx context.Context, attemptID, studentID string, reason attempts.Reason) (attempts.Attempt, error)
	Result(ctx context.Context, attemptID, studentID string) (attempts.Result, error)
	Monitor(ctx context.Context, assignmentID string) (attempts.Monitor, error)
	Extend(ctx context.Context, req attempts.Request, attemptID string, minutes int, reason string) (attempts.Attempt, error)
	Flag(ctx context.Context, req attempts.Request, attemptID string, flagged bool, reason string) (attempts.Attempt, error)
	Reset(ctx context.Context, req attempts.Request, attemptID, reason string) (attempts.Attempt, error)
	Void(ctx context.Context, req attempts.Request, attemptID, reason string) (attempts.Attempt, error)
}

// ReviewService is the slice of internal/review the handlers use.
type ReviewService interface {
	Get(ctx context.Context, attemptID string) (review.Review, error)
	Grade(ctx context.Context, attemptID, graderID string, items []review.Item) (attempts.Score, error)
	Finish(ctx context.Context, attemptID string) (attempts.Attempt, error)
	SetNote(ctx context.Context, attemptID string, note *string) error
	AnswersForQuestion(ctx context.Context, assignmentID, questionID string) (review.ByQuestion, error)
}

// IntegrityService is the slice of internal/integrity the handlers use.
type IntegrityService interface {
	Timeline(ctx context.Context, attemptID string) (integrity.Timeline, error)
}

// StudentsService is what the attempt review still reads from identity until attempts moves.
type StudentsService interface {
	Get(ctx context.Context, id string) (identitydomain.Student, error)
}

// TokenVerifier checks an access token. Separate from AuthService because the
// auth middleware needs it before any handler runs, and because verification is
// pure -- no database, no state.
type TokenVerifier interface {
	Verify(raw string) (*identityapp.Claims, error)
}

// verifyAccessToken adapts the token issuer to what the middleware wants.
//
// A nil verifier is a wiring mistake, not a caller error: refusing every
// request is the only safe response, and it is loud enough to find in one run.
func (d Deps) verifyAccessToken(bearer string) (httpx.Principal, error) {
	if d.Tokens == nil {
		return httpx.Principal{}, errors.New("no token verifier configured")
	}
	claims, err := d.Tokens.Verify(bearer)
	if err != nil {
		return httpx.Principal{}, err
	}
	return httpx.Principal{UserID: claims.Subject, Role: claims.Role}, nil
}
