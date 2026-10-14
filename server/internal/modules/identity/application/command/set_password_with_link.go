package command

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/application/model"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/opt"
	"unicode/utf8"
)

// SetPasswordWithLink uses a set-password link: it sets the password of the
// account the token names and signs the account in.
type SetPasswordWithLink struct {
	Token       string
	NewPassword string
	UserAgent   string
	IP          string
	GeoLabel    string
}

type SetPasswordWithLinkHandler struct {
	*support.Service
}

// Handle fails with domain.ErrPasswordSignInDisabled, before it looks at the
// token, while the centre has password sign-in off, and with
// domain.ErrSetPasswordLinkInvalid for a link that cannot be used. An invalid
// link is refused before any password is hashed. The password is set, the
// account's other links and sessions are ended and its cached permissions are
// dropped before the new session is issued.
func (s SetPasswordWithLinkHandler) Handle(ctx context.Context, cmd SetPasswordWithLink) (model.Session, error) {
	tokenHash, err := s.usableLink(ctx, cmd)
	if err != nil {
		return model.Session{}, err
	}
	newHash, err := domain.Passwords.Hash(ctx, cmd.NewPassword)
	if err != nil {
		return model.Session{}, fmt.Errorf("hash new password: %w", err)
	}
	claimed, err := s.Links.Claim(ctx, domain.ClaimLink{
		TokenHash: tokenHash,
		NewHash:   newHash,
		Now:       s.Now(),
		IP:        opt.String(cmd.IP),
		UserAgent: opt.String(cmd.UserAgent),
	})
	if err != nil {
		return model.Session{}, err
	}
	s.Principals.Forget(claimed.UserID)
	return s.signIn(ctx, claimed.UserID, support.Origin{UserAgent: cmd.UserAgent, IP: cmd.IP, GeoLabel: cmd.GeoLabel})
}

func (s SetPasswordWithLinkHandler) usableLink(ctx context.Context, cmd SetPasswordWithLink) ([]byte, error) {
	policy, err := s.SignInPolicy(ctx)
	if err != nil {
		return nil, fmt.Errorf("read the sign-in policy: %w", err)
	}
	if !policy.Password {
		return nil, domain.ErrPasswordSignInDisabled
	}
	switch n := utf8.RuneCountInString(cmd.NewPassword); {
	case n < domain.MinPasswordLength:
		return nil, domain.ErrPasswordTooShort
	case n > domain.MaxPasswordLength:
		return nil, domain.ErrPasswordTooLong
	}
	if s.Links == nil {
		return nil, domain.ErrSetPasswordLinkInvalid
	}
	hash := domain.HashSetPasswordToken(cmd.Token)
	link, err := s.Links.Find(ctx, hash)
	if err != nil {
		return nil, err
	}
	if !link.Usable(s.Now()) {
		return nil, domain.ErrSetPasswordLinkInvalid
	}
	return hash, nil
}

func (s SetPasswordWithLinkHandler) signIn(ctx context.Context, userID string, origin support.Origin) (model.Session, error) {
	for attempt := 0; ; attempt++ {
		user, err := s.Users.FindUserByID(ctx, userID)
		if err != nil {
			return model.Session{}, fmt.Errorf("reload account whose password was set: %w", err)
		}
		if user.Disabled() {
			return model.Session{}, domain.ErrSetPasswordLinkInvalid
		}
		session, err := s.IssueSession(ctx, user, origin, domain.SignInSetPasswordLink)
		if errors.Is(err, domain.ErrAccountChanged) && attempt == 0 {
			continue
		}
		if errors.Is(err, domain.ErrAccountChanged) {
			return model.Session{}, domain.ErrSetPasswordLinkInvalid
		}
		return session, err
	}
}
