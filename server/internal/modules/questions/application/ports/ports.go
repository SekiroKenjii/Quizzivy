package ports

import (
	"context"
)

// Service validates a question against the rules a schema cannot express, then
// writes it.
// MediaKinds answers what kind of asset an id names, or ErrMediaNotFound; the media module provides it.
type MediaKinds interface {
	Kind(ctx context.Context, assetID string) (string, error)
}
