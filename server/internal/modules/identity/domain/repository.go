package domain

import (
	"context"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/paging"
	"time"
)

// Users persists accounts, their provider identities and their refresh-token families.
type Users interface {
	FindUserByEmail(ctx context.Context, email string) (User, error)
	FindUserByID(ctx context.Context, id string) (User, error)
	FindUserByProviderIdentity(ctx context.Context, provider, providerUserID string) (User, error)
	CreateRefreshToken(ctx context.Context, in RefreshTokenRecord) error
	LinkIdentity(ctx context.Context, userID, provider, providerUserID, emailAtLink string) error
	UnlinkIdentity(ctx context.Context, userID, provider string) (bool, error)
	WriteAudit(ctx context.Context, e audit.Entry) error
	Rotate(ctx context.Context, tokenHash []byte, next RefreshTokenRecord, now time.Time) (RotateResult, error)
	RevokeFamilyByToken(ctx context.Context, tokenHash []byte, now time.Time) (string, error)
	DeleteExpired(ctx context.Context, before time.Time) (int64, error)
	ChangePassword(ctx context.Context, in ChangePasswordRecord) error
	Rename(ctx context.Context, in RenameRecord) (User, error)
}

// Students is the roster of student accounts, each teacher reaching the
// students visibility.StudentIDs gives them, or every one under scope.all.
// Another teacher's student answers exactly as a missing one does.
type Students interface {
	Delete(ctx context.Context, req WriteRequest, id string, now time.Time) error
	List(ctx context.Context, q StudentQuery) ([]Student, paging.Page, error)
	Get(ctx context.Context, scope access.Scope, id string) (Student, error)
	Account(ctx context.Context, id string) (Account, error)
	Facets(ctx context.Context, q StudentQuery) (StudentFacets, error)
	Create(ctx context.Context, req WriteRequest, in NewStudent) (Student, error)
	Update(ctx context.Context, req WriteRequest, in StudentPatch) (Student, error)
	ResetPassword(ctx context.Context, req WriteRequest, id, hash string, now time.Time) error
}
