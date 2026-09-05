package domain

import (
	"context"

	"quizzivy/internal/shared/paging"
)

// Repository persists asset rows and answers who references them.
type Repository interface {
	Insert(ctx context.Context, in InsertInput) (Asset, error)
	Get(ctx context.Context, id string) (Asset, error)
	CountByChecksum(ctx context.Context, checksum []byte) (int, error)
	List(ctx context.Context, in ListInput) ([]Asset, paging.Page, error)
	TotalBytes(ctx context.Context, kind *Kind) (int64, error)
	SoftDelete(ctx context.Context, in DeleteInput) error
	ReferencesFor(ctx context.Context, assetIDs []string) (map[string][]TestRef, error)
	ReachableByStudent(ctx context.Context, studentID, assetID string) (bool, error)
}
