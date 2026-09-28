package query

import (
	"context"
	"quizzivy/internal/modules/media/application/internal/support"
	"quizzivy/internal/modules/media/domain"
)

// SignedURL mints a fresh URL for an asset, per request (§11.2). It is not
// scoped: callers pass only an asset reached through a parent the caller
// already read under its own scope, never one resolved from an id taken from
// a request, which is what Readable is for.
type SignedURL struct {
	Asset domain.Asset
}

type SignedURLHandler struct {
	*support.Service
}

func (s SignedURLHandler) Handle(ctx context.Context, q SignedURL) (string, error) {
	return s.Object.SignedURL(ctx, q.Asset.StorageKey, s.TTL)
}
