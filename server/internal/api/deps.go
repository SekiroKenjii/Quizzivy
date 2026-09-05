package api

import (
	"context"
	"errors"
	identityapp "quizzivy/internal/modules/identity/application"
	identitydomain "quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/paging"
	"time"

	"quizzivy/internal/modules/assignments"
	"quizzivy/internal/modules/attempts"
	"quizzivy/internal/modules/integrity"
	mediaapp "quizzivy/internal/modules/media/application"
	mediadomain "quizzivy/internal/modules/media/domain"
	"quizzivy/internal/modules/review"
	"quizzivy/internal/modules/tests"
	"quizzivy/internal/modules/tests/publish"
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

// TestsService is the slice of internal/tests the handlers use.
type TestsService interface {
	List(ctx context.Context, in tests.ListInput) ([]tests.Test, paging.Page, error)
	Facets(ctx context.Context, in tests.ListInput) (tests.StatusFacets, error)
	Tags(ctx context.Context, in tests.ListInput) ([]string, error)
	Get(ctx context.Context, id string) (tests.Test, error)
	Create(ctx context.Context, req tests.Request, title string, description *string) (tests.Test, error)
	Update(ctx context.Context, req tests.Request, in tests.UpdateInput) (tests.Test, error)
	Duplicate(ctx context.Context, req tests.Request) (tests.Test, error)
	ListVersions(ctx context.Context, testID string) ([]tests.Version, error)
	Preview(ctx context.Context, testID string, version int) (int, []tests.PreviewQuestion, error)
}

// AssignmentsService is the slice of internal/assignments the handlers use.
type AssignmentsService interface {
	List(ctx context.Context, in assignments.ListInput) ([]assignments.Assignment, paging.Page, error)
	Get(ctx context.Context, id string) (assignments.Assignment, error)
	ForStudent(ctx context.Context, studentID string, now time.Time) (assignments.StudentSections, error)
	StudentDetail(ctx context.Context, id, studentID string) (assignments.StudentDetail, error)
	Create(ctx context.Context, req assignments.Request, in assignments.WriteInput) (assignments.Assignment, error)
	Update(ctx context.Context, req assignments.Request, in assignments.WriteInput) (assignments.Assignment, error)
	Facets(ctx context.Context, in assignments.ListInput) (assignments.Facets, error)
	Reopen(ctx context.Context, req assignments.Request, closesAt time.Time, reason string, now time.Time) (assignments.Assignment, error)
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

// PublishService is the slice of internal/tests/publish the handlers use.
type PublishService interface {
	Publish(ctx context.Context, req publish.Request) (publish.Version, error)
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
