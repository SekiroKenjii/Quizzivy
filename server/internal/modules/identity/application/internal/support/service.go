package support

import (
	"context"
	"fmt"
	classescommand "quizzivy/internal/modules/classes/application/command"
	classesdomain "quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/modules/identity/application/model"
	"quizzivy/internal/modules/identity/application/ports"
	"quizzivy/internal/modules/identity/application/token"
	"quizzivy/internal/modules/identity/domain"
	"time"

	"github.com/google/uuid"
)

// Service carries what the service handlers share: their ports and the helpers they call.
type Service struct {
	Users      domain.Users
	Tokens     *token.Issuer
	RefreshTTL time.Duration
	Now        func() time.Time
	Google     ports.GoogleProvider
	Enroller   ports.SelfEnroller
}

// SetGoogle wires the provider. Nil leaves Google sign-in unavailable rather
// than half-configured.
func (s *Service) SetGoogle(p ports.GoogleProvider, enroller ports.SelfEnroller) {
	s.Google = p
	s.Enroller = enroller
}

func (s *Service) VerifiedIdentity(ctx context.Context, code, verifier, redirectURI string) (model.GoogleIdentity, error) {
	rawIDToken, err := s.Google.Exchange(ctx, code, verifier, redirectURI)
	if err != nil {
		return model.GoogleIdentity{}, err
	}
	identity, err := s.Google.Verify(ctx, rawIDToken)
	if err != nil {
		return model.GoogleIdentity{}, err
	}
	if !identity.EmailVerified {
		return model.GoogleIdentity{}, domain.ErrGoogleEmailUnverified
	}
	return identity, nil
}

func (s *Service) LinkAndReload(ctx context.Context, userID string, identity model.GoogleIdentity) (domain.User, error) {
	if err := s.Users.LinkIdentity(ctx, userID, "google", identity.Subject, identity.Email); err != nil {
		return domain.User{}, err
	}
	user, err := s.Users.FindUserByID(ctx, userID)
	if err != nil {
		return domain.User{}, fmt.Errorf("reload linked user: %w", err)
	}
	return user, nil
}

func (s *Service) EnrolByCode(ctx context.Context, identity model.GoogleIdentity, in GoogleSignInInput) (model.GoogleSignInResult, error) {
	if s.Enroller == nil {
		return model.GoogleSignInResult{}, domain.ErrSelfEnrolNotAvailable
	}
	result, err := s.Enroller.Handle(ctx, classescommand.EnrolNewMember{
		Member: classesdomain.NewMember{
			Email:          identity.Email,
			FullName:       identity.Name,
			Provider:       "google",
			ProviderUserID: identity.Subject,
		},
		Code: in.JoinCode,
		Meta: classesdomain.Meta{IP: in.IP, UserAgent: in.UserAgent},
	})
	if err != nil {
		return model.GoogleSignInResult{}, err
	}
	if result.Outcome != classesdomain.PreviewOK {
		return model.GoogleSignInResult{}, domain.JoinCodeRejected{Outcome: result.Outcome}
	}
	created, err := s.Users.FindUserByID(ctx, result.UserID)
	if err != nil {
		return model.GoogleSignInResult{}, fmt.Errorf("load enrolled member: %w", err)
	}
	return s.GoogleSession(ctx, created, in, &result.Class)
}

func (s *Service) GoogleSession(ctx context.Context, user domain.User, in GoogleSignInInput, class *classesdomain.EnrolledClass) (model.GoogleSignInResult, error) {
	if user.Disabled() {
		return model.GoogleSignInResult{}, domain.ErrAccountDisabled
	}
	session, err := s.IssueSession(ctx, user, in.UserAgent, in.IP)
	if err != nil {
		return model.GoogleSignInResult{}, err
	}
	return model.GoogleSignInResult{Session: session, EnrolledClass: class}, nil
}

func (s *Service) AlreadyLinked(ctx context.Context, user domain.User, identity model.GoogleIdentity) (domain.User, error) {
	existing, err := s.Users.FindUserByProviderIdentity(ctx, "google", identity.Subject)
	if err == nil && existing.ID == user.ID {
		return user, nil
	}
	return domain.User{}, domain.ErrIdentityAlreadyLinked
}

func NewService(users domain.Users, tokens *token.Issuer, refreshTTL time.Duration) *Service {
	return &Service{Users: users, Tokens: tokens, RefreshTTL: refreshTTL, Now: time.Now}
}

// SetClock replaces the time source. Tests only.
func (s *Service) SetClock(now func() time.Time) { s.Now = now }

func (s *Service) IssueSession(ctx context.Context, user domain.User, userAgent, ip string) (model.Session, error) {
	access, err := s.Tokens.Issue(user.ID, user.Role)
	if err != nil {
		return model.Session{}, fmt.Errorf("issue access token: %w", err)
	}

	refresh, hash, err := NewRefreshToken()
	if err != nil {
		return model.Session{}, err
	}

	now := s.Now()
	rec := domain.RefreshTokenRecord{
		UserID:    user.ID,
		FamilyID:  uuid.NewString(),
		TokenHash: hash,
		IssuedAt:  now,
		ExpiresAt: now.Add(s.RefreshTTL),
	}
	if userAgent != "" {
		rec.UserAgent = &userAgent
	}
	if ip != "" {
		rec.IP = &ip
	}
	if err := s.Users.CreateRefreshToken(ctx, rec); err != nil {
		return model.Session{}, fmt.Errorf("store refresh token: %w", err)
	}

	return model.Session{
		AccessToken:  access,
		ExpiresIn:    int(s.Tokens.TTL().Seconds()),
		RefreshToken: refresh,
		User:         user,
	}, nil
}
