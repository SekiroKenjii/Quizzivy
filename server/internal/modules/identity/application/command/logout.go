package command

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/cqrs"
	"time"
)

// LogoutBudget is how long a logout's revoke may run, however soon the request
// that asked for it ends.
const LogoutBudget = 10 * time.Second

// Logout revokes the whole family the presented token belongs to, ending every
// session descended from that login rather than only the current one. The
// revoke runs on a context that keeps the request's values and not its end,
// under LogoutBudget, so a tab closed or a connection lost during sign-out
// still ends the session: the refresh cookie stays in the browser of a client
// that never saw the answer, and a family left live would sign the next visitor
// of that machine in.
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

	revoke, cancel := context.WithTimeout(context.WithoutCancel(ctx), LogoutBudget)
	defer cancel()
	_, err := s.Users.RevokeFamilyByToken(revoke, presented[:], s.Now())
	if errors.Is(err, domain.ErrRefreshTokenNotFound) {
		return cqrs.Nothing{}, nil
	}
	if err != nil {
		return cqrs.Nothing{}, fmt.Errorf("logout: %w", err)
	}
	return cqrs.Nothing{}, nil
}
