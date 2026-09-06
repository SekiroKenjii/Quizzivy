package command

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/cqrs"
)

// Logout revokes the whole family the presented token belongs to, ending every
// session descended from that login rather than only the current one.
type Logout struct {
	Token string
}

type LogoutHandler struct {
	*support.Service
}

func (s LogoutHandler) Handle(ctx context.Context, cmd Logout) (cqrs.Nothing, error) {
	if cmd.Token == "" {
		return cqrs.Nothing{}, domain.ErrRefreshRejected
	}
	presented := sha256.Sum256([]byte(cmd.Token))

	_, err := s.Users.RevokeFamilyByToken(ctx, presented[:], s.Now())
	if errors.Is(err, domain.ErrRefreshTokenNotFound) {
		return cqrs.Nothing{}, nil
	}
	if err != nil {
		return cqrs.Nothing{}, fmt.Errorf("logout: %w", err)
	}
	return cqrs.Nothing{}, nil
}
