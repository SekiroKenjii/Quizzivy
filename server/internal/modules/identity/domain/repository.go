package domain

import (
	"context"
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
}

// Students is the teacher's roster of student accounts.
type Students interface {
	List(ctx context.Context, q StudentQuery) ([]Student, paging.Page, error)
	Get(ctx context.Context, id string) (Student, error)
	Facets(ctx context.Context, q StudentQuery) (StudentFacets, error)
	Create(ctx context.Context, req WriteRequest, in NewStudent) (Student, error)
	Update(ctx context.Context, req WriteRequest, in StudentPatch) (Student, error)
	ResetPassword(ctx context.Context, req WriteRequest, id, hash string, now time.Time) error
}
