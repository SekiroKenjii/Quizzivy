package query

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
)

// CurrentUser backs GET /auth/me (§7).
type CurrentUser struct {
	UserID string
}

type CurrentUserHandler struct {
	*support.Service
}

func (s CurrentUserHandler) Handle(ctx context.Context, q CurrentUser) (domain.User, error) {
	user, err := s.Users.FindUserByID(ctx, q.UserID)
	if err != nil {
		return domain.User{}, err
	}
	if user.Disabled() {
		return domain.User{}, domain.ErrAccountDisabled
	}
	return user, nil
}
