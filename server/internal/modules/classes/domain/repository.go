package domain

import (
	"context"

	"quizzivy/internal/shared/paging"
)

// Repository persists classes, their roster and their join codes.
type Repository interface {
	Get(ctx context.Context, classID string) (Class, error)
	List(ctx context.Context, in ListInput) ([]Class, paging.Page, error)
	ListMine(ctx context.Context, userID string) ([]MyClass, error)
	Members(ctx context.Context, classID string, in MembersInput) ([]Member, paging.Page, error)
	Facets(ctx context.Context, query string) (Facets, error)
	Create(ctx context.Context, in CreateInput) (Class, error)
	Update(ctx context.Context, classID string, in UpdateInput) (Class, error)
	Archive(ctx context.Context, in ArchiveInput) (Class, error)
	AddMember(ctx context.Context, in AddMemberInput) (Member, error)
	RemoveMember(ctx context.Context, in RemoveMemberInput) error
	Rotate(ctx context.Context, in RotateInput) (IssuedCode, error)
	Revoke(ctx context.Context, in RevokeInput) error
	ActiveCode(ctx context.Context, classID string) (*IssuedCode, error)
	Enrol(ctx context.Context, in EnrolInput) (EnrolResult, error)
	LookupByCodeHash(ctx context.Context, hash []byte) (*CodeRow, error)
}
