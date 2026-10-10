package command

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/cqrs"
	"quizzivy/internal/shared/opt"
)

// RevokeSession ends one live session of the caller. RefreshToken is the
// refresh cookie of the request, which names the caller's own session.
type RevokeSession struct {
	UserID       string
	FamilyID     string
	RefreshToken string
	IP           string
	UserAgent    string
}

type RevokeSessionHandler struct {
	*support.Service
}

// Handle revokes the family, moves the caller's session epoch and then, once
// that has committed, forgets the caller's cached principal on this machine.
func (s RevokeSessionHandler) Handle(ctx context.Context, cmd RevokeSession) (cqrs.Nothing, error) {
	err := s.Users.RevokeSession(ctx, domain.RevokeSessionRecord{
		UserID:           cmd.UserID,
		FamilyID:         cmd.FamilyID,
		CurrentTokenHash: support.TokenHash(cmd.RefreshToken),
		Now:              s.Now(),
		IP:               opt.String(cmd.IP),
		UserAgent:        opt.String(cmd.UserAgent),
	})
	if err != nil {
		return cqrs.Nothing{}, err
	}
	s.Principals.Forget(cmd.UserID)
	return cqrs.Nothing{}, nil
}

// RevokeOtherSessions ends every live session of the caller except their own.
// RefreshToken is the refresh cookie of the request.
type RevokeOtherSessions struct {
	UserID       string
	RefreshToken string
	IP           string
	UserAgent    string
}

type RevokeOtherSessionsHandler struct {
	*support.Service
}

// Handle returns how many sessions it ended. Only when it ended one has the
// caller's session epoch moved, and only then does it forget the caller's
// cached principal.
func (s RevokeOtherSessionsHandler) Handle(ctx context.Context, cmd RevokeOtherSessions) (int, error) {
	revoked, err := s.Users.RevokeOtherSessions(ctx, domain.RevokeOtherSessionsRecord{
		UserID:           cmd.UserID,
		CurrentTokenHash: support.TokenHash(cmd.RefreshToken),
		Now:              s.Now(),
		IP:               opt.String(cmd.IP),
		UserAgent:        opt.String(cmd.UserAgent),
	})
	if err != nil {
		return 0, err
	}
	if revoked > 0 {
		s.Principals.Forget(cmd.UserID)
	}
	return revoked, nil
}
