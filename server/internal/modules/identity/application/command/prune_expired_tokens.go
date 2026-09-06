package command

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
)

// PruneExpiredTokens deletes refresh tokens past their expiry. Intended for a
// scheduled call; returns how many rows went.
type PruneExpiredTokens struct {
}

type PruneExpiredTokensHandler struct {
	*support.Service
}

func (s PruneExpiredTokensHandler) Handle(ctx context.Context, _ PruneExpiredTokens) (int64, error) {
	return s.Users.DeleteExpired(ctx, s.Now())
}
