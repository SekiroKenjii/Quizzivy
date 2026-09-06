package command

import (
	"context"
	"crypto/sha256"
	"fmt"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/cqrs"
	"quizzivy/internal/shared/opt"
	"unicode/utf8"
)

// ChangePassword verifies the current password, replaces it, clears
// must_change_password, and revokes every other refresh family (§5.4).
type ChangePassword struct {
	UserID           string
	CurrentPassword  string
	NewPassword      string
	KeepRefreshToken string
	IP               string
	UserAgent        string
}

type ChangePasswordHandler struct {
	*support.Service
}

func (s ChangePasswordHandler) Handle(ctx context.Context, cmd ChangePassword) (cqrs.Nothing, error) {
	switch n := utf8.RuneCountInString(cmd.NewPassword); {
	case n < domain.MinPasswordLength:
		return cqrs.Nothing{}, domain.ErrPasswordTooShort
	case n > domain.MaxPasswordLength:
		return cqrs.Nothing{}, domain.ErrPasswordTooLong
	}

	user, err := s.Users.FindUserByID(ctx, cmd.UserID)
	if err != nil {
		return cqrs.Nothing{}, err
	}
	if user.Disabled() {
		return cqrs.Nothing{}, domain.ErrAccountDisabled
	}
	if !user.HasPassword() {
		return cqrs.Nothing{}, domain.ErrNoPasswordSet
	}

	if !user.MustChangePassword {
		ok, err := domain.Passwords.Verify(ctx, cmd.CurrentPassword, *user.PasswordHash)
		if err != nil {
			return cqrs.Nothing{}, fmt.Errorf("verify current password for %s: %w", user.ID, err)
		}
		if !ok {
			return cqrs.Nothing{}, domain.ErrInvalidCredentials
		}
	}

	newHash, err := domain.Passwords.Hash(ctx, cmd.NewPassword)
	if err != nil {
		return cqrs.Nothing{}, fmt.Errorf("hash new password: %w", err)
	}

	var keepHash []byte
	if cmd.KeepRefreshToken != "" {
		sum := sha256.Sum256([]byte(cmd.KeepRefreshToken))
		keepHash = sum[:]
	}

	return cqrs.Nothing{}, s.Users.ChangePassword(ctx, domain.ChangePasswordRecord{
		UserID:        user.ID,
		NewHash:       newHash,
		KeepTokenHash: keepHash,
		Now:           s.Now(),
		IP:            opt.String(cmd.IP),
		UserAgent:     opt.String(cmd.UserAgent),
	})
}
