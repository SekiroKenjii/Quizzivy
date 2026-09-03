package api

import (
	"context"
	"errors"
	"quizzivy/internal/paging"
	"time"

	"quizzivy/internal/assignments"
	"quizzivy/internal/attempts"
	"quizzivy/internal/auth"
	"quizzivy/internal/classes"
	"quizzivy/internal/dashboard"
	"quizzivy/internal/httpx"
	"quizzivy/internal/join"
	"quizzivy/internal/media"
	"quizzivy/internal/questions"
	"quizzivy/internal/students"
	"quizzivy/internal/tests"
	"quizzivy/internal/tests/publish"
)

// DB is the slice of the pool handlers need. An interface rather than the
// concrete pool so tests can substitute one without a live database.
type DB interface {
	Ping(ctx context.Context) error
}

// AuthService is the slice of internal/auth the handlers use. An interface so
// a handler test can supply a fake without a database.
type AuthService interface {
	Login(ctx context.Context, in auth.LoginInput) (auth.Session, error)
	Refresh(ctx context.Context, in auth.RefreshInput) (auth.RefreshResult, error)
	Logout(ctx context.Context, token string) error
	CurrentUser(ctx context.Context, userID string) (auth.User, error)
	ChangePassword(ctx context.Context, in auth.ChangePasswordInput) error
	GoogleSignIn(ctx context.Context, in auth.GoogleSignInInput) (auth.GoogleSignInResult, error)
	LinkGoogle(ctx context.Context, in auth.LinkGoogleInput) (auth.User, error)
	UnlinkGoogle(ctx context.Context, userID, ip, userAgent string) error
	NewTemporaryPassword(ctx context.Context) (password, hash string, err error)
}

// JoinService is the slice of internal/join the handlers use.
type JoinService interface {
	Rotate(ctx context.Context, req join.RotateRequest) (join.Rotated, error)
	Revoke(ctx context.Context, req join.RevokeRequest) error
	Preview(ctx context.Context, rawCode string) (join.PreviewResult, error)
	EnrolExisting(ctx context.Context, userID, rawCode string, meta join.Meta) (join.EnrolResult, error)
}

// ClassesService is the slice of internal/classes the handlers use.
type ClassesService interface {
	Get(ctx context.Context, classID string) (classes.Class, error)
	List(ctx context.Context, in classes.ListInput) ([]classes.Class, paging.Page, error)
	ListMine(ctx context.Context, userID string) ([]classes.Class, error)
	Members(ctx context.Context, classID string, in classes.MembersInput) ([]classes.Member, paging.Page, error)
	Update(ctx context.Context, classID string, in classes.UpdateInput) (classes.Class, error)
	RemoveMember(ctx context.Context, classID, userID, actorID, ip, userAgent string) error
	AddMember(ctx context.Context, classID, userID, actorID, ip, userAgent string) (classes.Member, error)
}

// MediaService is the slice of internal/media the handlers use.
type MediaService interface {
	Upload(ctx context.Context, in media.UploadInput) (media.Asset, error)
	SignedURL(ctx context.Context, asset media.Asset) (string, error)
	List(ctx context.Context, in media.ListInput) ([]media.Asset, paging.Page, error)
	Delete(ctx context.Context, in media.DeleteInput) error
	MintForStudent(ctx context.Context, studentID, assetID string) (media.SignedURLResult, error)
	// Get resolves one asset, so a question can render its attachment.
	Get(ctx context.Context, id string) (media.Asset, error)
	SignedURLTTL() time.Duration
}

// QuestionsService is the slice of internal/questions the handlers use.
type QuestionsService interface {
	List(ctx context.Context, in questions.ListInput) ([]questions.Question, paging.Page, error)
	Facets(ctx context.Context, in questions.ListInput) (questions.TypeFacets, error)
	Get(ctx context.Context, id string) (questions.Question, error)
	Create(ctx context.Context, req questions.WriteRequest) (questions.Question, error)
	Update(ctx context.Context, req questions.WriteRequest) (questions.Question, error)
	Delete(ctx context.Context, req questions.WriteRequest) error
	AddTags(ctx context.Context, ids []string, tags []string) (int, error)
	Tags(ctx context.Context, in questions.ListInput) ([]string, error)
	Counts(ctx context.Context, in questions.ListInput) (int, int, error)
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
}

// AttemptsService is the slice of internal/attempts the handlers use.
type AttemptsService interface {
	StartOrResume(ctx context.Context, assignmentID, studentID string) (attempts.Session, error)
	Get(ctx context.Context, attemptID, studentID string) (attempts.Session, error)
	Save(ctx context.Context, in attempts.SaveInput) (attempts.SaveResult, error)
	RecordPlay(ctx context.Context, attemptID, studentID, questionID string) (attempts.Plays, error)
	Flush(ctx context.Context, in attempts.FlushInput) error
	Submit(ctx context.Context, attemptID, studentID string, reason attempts.Reason) (attempts.Attempt, error)
}

// StudentsService is the slice of internal/students the handlers use.
type StudentsService interface {
	List(ctx context.Context, in students.ListInput) ([]students.Student, paging.Page, error)
	Facets(ctx context.Context, in students.ListInput) (students.Facets, error)
	Get(ctx context.Context, id string) (students.Student, error)
	Create(ctx context.Context, req students.Request, in students.CreateInput) (students.Student, error)
	Update(ctx context.Context, req students.Request, in students.UpdateInput) (students.Student, error)
	ResetPassword(ctx context.Context, req students.Request, id, hash string, now time.Time) error
}

// DashboardService is the slice of internal/dashboard the handlers use.
type DashboardService interface {
	Get(ctx context.Context) (dashboard.Summary, error)
}

// PublishService is the slice of internal/tests/publish the handlers use.
type PublishService interface {
	Publish(ctx context.Context, req publish.Request) (publish.Version, error)
}

// TokenVerifier checks an access token. Separate from AuthService because the
// auth middleware needs it before any handler runs, and because verification is
// pure -- no database, no state.
type TokenVerifier interface {
	Verify(raw string) (*auth.Claims, error)
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
