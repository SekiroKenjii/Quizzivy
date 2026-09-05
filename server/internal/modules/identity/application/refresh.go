package application

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	"quizzivy/internal/modules/identity/domain"
)

type RefreshInput struct {
	Token     string
	UserAgent string
	IP        string
}

// RefreshResult carries the new access token and the replacement refresh
// token. The refresh token goes into a Set-Cookie header and nowhere else.
type RefreshResult struct {
	AccessToken  string
	ExpiresIn    int
	RefreshToken string
	User         domain.User
}

// Refresh rotates a refresh token (§5.2).
func (s *Service) Refresh(ctx context.Context, in RefreshInput) (RefreshResult, error) {
	if in.Token == "" {
		return RefreshResult{}, domain.ErrRefreshRejected
	}
	presented := sha256.Sum256([]byte(in.Token))

	successor, successorHash, err := newRefreshToken()
	if err != nil {
		return RefreshResult{}, err
	}

	now := s.now()
	next := domain.RefreshTokenRecord{
		TokenHash: successorHash,
		IssuedAt:  now,
		ExpiresAt: now.Add(s.refreshTTL),
	}
	if in.UserAgent != "" {
		next.UserAgent = &in.UserAgent
	}
	if in.IP != "" {
		next.IP = &in.IP
	}

	res, err := s.users.Rotate(ctx, presented[:], next, now)
	if err != nil {
		return RefreshResult{}, fmt.Errorf("rotate refresh token: %w", err)
	}
	switch res.Outcome {
	case domain.RotateOK:
	case domain.RotateReused:

		return RefreshResult{}, domain.ErrRefreshReused
	default:
		return RefreshResult{}, domain.ErrRefreshRejected
	}

	access, err := s.tokens.Issue(res.User.ID, res.User.Role)
	if err != nil {
		return RefreshResult{}, fmt.Errorf("issue access token: %w", err)
	}

	return RefreshResult{
		AccessToken:  access,
		ExpiresIn:    int(s.tokens.TTL().Seconds()),
		RefreshToken: successor,
		User:         res.User,
	}, nil
}

// Logout revokes the whole family the presented token belongs to, ending every
// session descended from that login rather than only the current one.
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return domain.ErrRefreshRejected
	}
	presented := sha256.Sum256([]byte(token))

	_, err := s.users.RevokeFamilyByToken(ctx, presented[:], s.now())
	if errors.Is(err, domain.ErrRefreshTokenNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("logout: %w", err)
	}
	return nil
}

// PruneExpiredTokens deletes refresh tokens past their expiry. Intended for a
// scheduled call; returns how many rows went.
func (s *Service) PruneExpiredTokens(ctx context.Context) (int64, error) {
	return s.users.DeleteExpired(ctx, s.now())
}
