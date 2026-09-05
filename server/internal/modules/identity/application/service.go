package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"quizzivy/internal/modules/identity/domain"

	"github.com/google/uuid"
)

type Service struct {
	users      domain.Users
	tokens     *TokenIssuer
	refreshTTL time.Duration
	now        func() time.Time
	google     GoogleProvider
	enroller   SelfEnroller
}

func NewService(users domain.Users, tokens *TokenIssuer, refreshTTL time.Duration) *Service {
	return &Service{users: users, tokens: tokens, refreshTTL: refreshTTL, now: time.Now}
}

// SetClock replaces the time source. Tests only.
func (s *Service) SetClock(now func() time.Time) { s.now = now }

// Session is what a successful login produces.
type Session struct {
	AccessToken  string
	ExpiresIn    int
	RefreshToken string // opaque; goes into the httpOnly cookie, never a body
	User         domain.User
}

type LoginInput struct {
	Email     string
	Password  string
	UserAgent string
	IP        string
}

// Login verifies a password and mints a session.
//
// An unknown email still runs a full hash against dummyHash, so the response
// time does not reveal which accounts exist.
func (s *Service) Login(ctx context.Context, in LoginInput) (Session, error) {
	user, err := s.users.FindUserByEmail(ctx, in.Email)
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		domain.BurnPasswordTime(ctx, in.Password)
		return Session{}, domain.ErrInvalidCredentials
	case err != nil:
		return Session{}, fmt.Errorf("look up user: %w", err)
	}

	if !user.HasPassword() {
		// A Google-only account (§5.1). Indistinguishable from a wrong password.
		domain.BurnPasswordTime(ctx, in.Password)
		return Session{}, domain.ErrInvalidCredentials
	}

	ok, err := domain.VerifyPassword(ctx, in.Password, *user.PasswordHash)
	if err != nil {
		return Session{}, fmt.Errorf("verify password for %s: %w", user.ID, err)
	}
	if !ok {
		return Session{}, domain.ErrInvalidCredentials
	}

	// Deliberately after verification, so the timing matches.
	if user.Disabled() {
		return Session{}, domain.ErrInvalidCredentials
	}

	return s.issueSession(ctx, user, in.UserAgent, in.IP)
}

func (s *Service) issueSession(ctx context.Context, user domain.User, userAgent, ip string) (Session, error) {
	access, err := s.tokens.Issue(user.ID, user.Role)
	if err != nil {
		return Session{}, fmt.Errorf("issue access token: %w", err)
	}

	refresh, hash, err := newRefreshToken()
	if err != nil {
		return Session{}, err
	}

	now := s.now()
	rec := domain.RefreshTokenRecord{
		UserID:    user.ID,
		FamilyID:  uuid.NewString(),
		TokenHash: hash,
		IssuedAt:  now,
		ExpiresAt: now.Add(s.refreshTTL),
	}
	if userAgent != "" {
		rec.UserAgent = &userAgent
	}
	if ip != "" {
		rec.IP = &ip
	}
	if err := s.users.CreateRefreshToken(ctx, rec); err != nil {
		return Session{}, fmt.Errorf("store refresh token: %w", err)
	}

	return Session{
		AccessToken:  access,
		ExpiresIn:    int(s.tokens.TTL().Seconds()),
		RefreshToken: refresh,
		User:         user,
	}, nil
}

// newRefreshToken returns the opaque token and its SHA-256 hash.
//
// 32 bytes from a CSPRNG: this is a bearer credential with a 30-day life, so it
// must not be guessable, and unlike the access token it carries no structure an
// attacker could exploit.
func newRefreshToken() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate refresh token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	return token, sum[:], nil
}
