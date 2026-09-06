package query

import (
	"context"
	"quizzivy/internal/modules/media/application/internal/support"
	"quizzivy/internal/modules/media/domain"
)

// SignedURL mints a fresh URL for an asset, per request (§11.2).
type SignedURL struct {
	Asset domain.Asset
}

type SignedURLHandler struct {
	*support.Service
}

func (s SignedURLHandler) Handle(ctx context.Context, q SignedURL) (string, error) {
	return s.Object.SignedURL(ctx, q.Asset.StorageKey, s.TTL)
}
