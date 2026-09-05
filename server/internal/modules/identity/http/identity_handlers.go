package http

import (
	"context"
	"time"

	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/paging"
)

// Auth is the slice of the accounts application this transport needs.
type Auth interface {
	Login(ctx context.Context, in application.LoginInput) (application.Session, error)
	Refresh(ctx context.Context, in application.RefreshInput) (application.RefreshResult, error)
	Logout(ctx context.Context, token string) error
	CurrentUser(ctx context.Context, userID string) (domain.User, error)
	ChangePassword(ctx context.Context, in application.ChangePasswordInput) error
	GoogleSignIn(ctx context.Context, in application.GoogleSignInInput) (application.GoogleSignInResult, error)
	LinkGoogle(ctx context.Context, in application.LinkGoogleInput) (domain.User, error)
	UnlinkGoogle(ctx context.Context, userID, ip, userAgent string) error
}

// Students is the teacher's roster of student accounts.
type Students interface {
	List(ctx context.Context, q domain.StudentQuery) ([]domain.Student, paging.Page, error)
	Facets(ctx context.Context, q domain.StudentQuery) (domain.StudentFacets, error)
	Get(ctx context.Context, id string) (domain.Student, error)
	Create(ctx context.Context, req domain.WriteRequest, in domain.NewStudent) (domain.Student, string, error)
	Update(ctx context.Context, req domain.WriteRequest, in domain.StudentPatch) (domain.Student, error)
	ResetPassword(ctx context.Context, req domain.WriteRequest, id string) (string, error)
}

type Identity struct {
	auth         Auth
	students     Students
	refreshTTL   time.Duration
	cookieSecure bool
}

func NewIdentity(auth Auth, students Students, refreshTTL time.Duration, cookieSecure bool) Identity {
	return Identity{auth: auth, students: students, refreshTTL: refreshTTL, cookieSecure: cookieSecure}
}
