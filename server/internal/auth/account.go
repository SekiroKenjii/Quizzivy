package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"unicode/utf8"
)

var (
	ErrAccountDisabled  = errors.New("account disabled")
	ErrNoPasswordSet    = errors.New("account has no password")
	ErrPasswordTooShort = errors.New("new password is too short")
	ErrPasswordTooLong  = errors.New("new password is too long")
)

// Password bounds from api/openapi.yaml. The maximum exists because Argon2id
// hashes whatever it is given, and a megabyte of "password" is a free way to
// burn CPU on an authenticated endpoint.
const (
	MinPasswordLength = 8
	MaxPasswordLength = 512
)

// CurrentUser backs GET /auth/me (§7).
//
// It reads the user afresh rather than trusting the access token's claims. A
// role change or a suspension made a minute ago must take effect now, not when
// the token happens to expire.
func (s *Service) CurrentUser(ctx context.Context, userID string) (User, error) {
	user, err := s.store.FindUserByID(ctx, userID)
	if err != nil {
		return User{}, err
	}
	if user.Disabled() {
		return User{}, ErrAccountDisabled
	}
	return user, nil
}

type ChangePasswordInput struct {
	UserID           string
	CurrentPassword  string
	NewPassword      string
	KeepRefreshToken string
	IP               string
	UserAgent        string
}

// ChangePassword verifies the current password, replaces it, clears
// must_change_password, and revokes every other refresh family (§5.4).
func (s *Service) ChangePassword(ctx context.Context, in ChangePasswordInput) error {
	switch n := utf8.RuneCountInString(in.NewPassword); {
	case n < MinPasswordLength:
		return ErrPasswordTooShort
	case n > MaxPasswordLength:
		return ErrPasswordTooLong
	}

	user, err := s.store.FindUserByID(ctx, in.UserID)
	if err != nil {
		return err
	}
	if user.Disabled() {
		return ErrAccountDisabled
	}
	if !user.HasPassword() {
		return ErrNoPasswordSet
	}

	ok, err := VerifyPassword(ctx, in.CurrentPassword, *user.PasswordHash)
	if err != nil {
		return fmt.Errorf("verify current password for %s: %w", user.ID, err)
	}
	if !ok {
		return ErrInvalidCredentials
	}

	newHash, err := HashPassword(ctx, in.NewPassword)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}

	var keepHash []byte
	if in.KeepRefreshToken != "" {
		sum := sha256.Sum256([]byte(in.KeepRefreshToken))
		keepHash = sum[:]
	}

	return s.store.ChangePassword(ctx, ChangePasswordRecord{
		UserID:        user.ID,
		NewHash:       newHash,
		KeepTokenHash: keepHash,
		Now:           s.now(),
		IP:            optional(in.IP),
		UserAgent:     optional(in.UserAgent),
	})
}

func optional(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
