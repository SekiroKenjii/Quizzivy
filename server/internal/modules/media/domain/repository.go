package domain

import (
	"context"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/paging"
)

// Repository persists asset rows and answers who references them. The library
// is an owner's assets that are neither deleted nor replaced: List, TotalBytes,
// Facets, Find and Update see only those. Usage and the quota check of Insert
// see them and the files that left it while something still names them. Get,
// Readable and SoftDelete also reach a replaced asset, but none reaches a purged
// one, whose object is gone. SoftDelete answers the file it retired.
type Repository interface {
	Retention
	FindReplacementTarget(context.Context, access.Scope, string) (ReplacementTarget, error)
	Replace(context.Context, ReplaceInput) (ReplaceResult, error)
	Insert(ctx context.Context, in InsertInput) (Asset, error)
	Get(ctx context.Context, id string) (Asset, error)
	Find(ctx context.Context, scope access.Scope, id string) (Asset, error)
	Update(ctx context.Context, in UpdateInput) (Asset, error)
	CountByChecksum(ctx context.Context, ownerID string, checksum []byte) (int, error)
	List(ctx context.Context, in ListInput) ([]Asset, paging.Page, error)
	TotalBytes(ctx context.Context, in ListInput) (int64, error)
	Facets(ctx context.Context, in ListInput) (Facets, error)
	Usage(ctx context.Context, scope access.Scope) (Usage, error)
	QuestionCounts(ctx context.Context, assetIDs []string) (map[string]int, error)
	SoftDelete(ctx context.Context, in DeleteInput) (Retired, error)
	ReferencesFor(ctx context.Context, assetIDs []string) (map[string][]TestRef, error)
	ReachableByStudent(ctx context.Context, studentID, assetID string) (bool, error)
	Readable(ctx context.Context, scope access.Scope, assetIDs []string) (map[string]Kind, error)
}
