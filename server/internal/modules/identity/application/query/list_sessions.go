package query

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
)

// ListSessions backs GET /auth/sessions: the caller's live sessions.
// RefreshToken is the refresh cookie of the request, which makes the caller's
// own session current.
type ListSessions struct {
	UserID       string
	RefreshToken string
}

type ListSessionsHandler struct {
	*support.Service
}

func (s ListSessionsHandler) Handle(ctx context.Context, q ListSessions) ([]domain.Session, error) {
	return s.Users.ListSessions(ctx, domain.SessionsQuery{
		UserID:           q.UserID,
		CurrentTokenHash: support.TokenHash(q.RefreshToken),
		Now:              s.Now(),
		Limit:            domain.MaxSessions,
	})
}
