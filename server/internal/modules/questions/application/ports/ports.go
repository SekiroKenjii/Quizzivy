package ports

import (
	"context"
	"quizzivy/internal/shared/access"
)

// MediaKinds answers the kind of an asset the scope may read, as the media
// module defines a readable asset, or ErrMediaNotFound alike for a missing,
// deleted or unreadable one; the media module provides it.
type MediaKinds interface {
	Kind(ctx context.Context, scope access.Scope, assetID string) (string, error)
}
