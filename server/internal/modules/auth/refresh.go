package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
)

// ErrRefreshRejected covers every ordinary refresh failure: no cookie, an
// unknown token, an expired one, or a suspended account. A suspended account is
// not called out, for the same reason Login does not call it out.
var ErrRefreshRejected = errors.New("refresh rejected")

// ErrRefreshReused is §5.2 reuse detection firing: the presented token had
// already been rotated, and the whole family has now been revoked.
//
// Kept distinct from ErrRefreshRejected for the VICTIM's benefit, not the
// attacker's. By the time this is returned the family is dead, so the fact
// leaks no access; but "someone else used your session" is a thing a student
// can act on, and "your session expired" is not.
var ErrRefreshReused = errors.New("refresh token reused")

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
	User         User
}

// Refresh rotates a refresh token (§5.2).
//
// Lookup is BY the SHA-256 of the presented token, so the database index does
// the matching and the plaintext is never stored. §13.5's constant-time
// requirement is about join codes, which are short and low-entropy; a refresh
// token is 256 bits from a CSPRNG, and an attacker cannot probe index timing
// without already holding a candidate token.
func (s *Service) Refresh(ctx context.Context, in RefreshInput) (RefreshResult, error) {
	if in.Token == "" {
		return RefreshResult{}, ErrRefreshRejected
	}
	presented := sha256.Sum256([]byte(in.Token))

	successor, successorHash, err := newRefreshToken()
	if err != nil {
		return RefreshResult{}, err
	}

	now := s.now()
	next := RefreshTokenRecord{
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

	res, err := s.store.Rotate(ctx, presented[:], next, now)
	if err != nil {
		return RefreshResult{}, fmt.Errorf("rotate refresh token: %w", err)
	}
	switch res.Outcome {
	case RotateOK:
	case RotateReused:
		// Rotate has already revoked the family and written the audit row.
		return RefreshResult{}, ErrRefreshReused
	default:
		return RefreshResult{}, ErrRefreshRejected
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
//
// Idempotent: an already-revoked or already-expired token still returns
// success. The caller's intent -- end this session -- is satisfied either way,
// and reporting "that token was not valid" would answer a question about
// another party's credential.
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return ErrRefreshRejected
	}
	presented := sha256.Sum256([]byte(token))

	_, err := s.store.RevokeFamilyByToken(ctx, presented[:], s.now())
	if errors.Is(err, ErrRefreshTokenNotFound) {
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
	return s.store.DeleteExpired(ctx, s.now())
}
