package application

import (
	"context"
	"crypto/sha256"
	"fmt"
	"unicode/utf8"

	"quizzivy/internal/modules/identity/domain"
)

// CurrentUser backs GET /auth/me (§7).
//
// It reads the user afresh rather than trusting the access token's claims. A
// role change or a suspension made a minute ago must take effect now, not when
// the token happens to expire.
func (s *Service) CurrentUser(ctx context.Context, userID string) (domain.User, error) {
	user, err := s.users.FindUserByID(ctx, userID)
	if err != nil {
		return domain.User{}, err
	}
	if user.Disabled() {
		return domain.User{}, domain.ErrAccountDisabled
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
	case n < domain.MinPasswordLength:
		return domain.ErrPasswordTooShort
	case n > domain.MaxPasswordLength:
		return domain.ErrPasswordTooLong
	}

	user, err := s.users.FindUserByID(ctx, in.UserID)
	if err != nil {
		return err
	}
	if user.Disabled() {
		return domain.ErrAccountDisabled
	}
	if !user.HasPassword() {
		return domain.ErrNoPasswordSet
	}

	// A forced change skips the current-password check.
	if !user.MustChangePassword {
		ok, err := domain.Passwords.Verify(ctx, in.CurrentPassword, *user.PasswordHash)
		if err != nil {
			return fmt.Errorf("verify current password for %s: %w", user.ID, err)
		}
		if !ok {
			return domain.ErrInvalidCredentials
		}
	}

	newHash, err := domain.Passwords.Hash(ctx, in.NewPassword)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}

	var keepHash []byte
	if in.KeepRefreshToken != "" {
		sum := sha256.Sum256([]byte(in.KeepRefreshToken))
		keepHash = sum[:]
	}

	return s.users.ChangePassword(ctx, domain.ChangePasswordRecord{
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
