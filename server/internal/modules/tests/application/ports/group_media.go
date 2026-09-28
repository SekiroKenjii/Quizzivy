package ports

import (
	"context"
	"quizzivy/internal/shared/access"
)

// GroupMediaKinds resolves attachment kinds without trusting a caller's claimed
// asset type, for assets the scope may read; a missing, deleted or unreadable
// asset answers alike.
type GroupMediaKinds interface {
	Kind(ctx context.Context, scope access.Scope, assetID string) (string, error)
}
