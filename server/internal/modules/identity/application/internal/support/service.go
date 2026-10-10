package support

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	classescommand "quizzivy/internal/modules/classes/application/command"
	classesdomain "quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/modules/identity/application/model"
	"quizzivy/internal/modules/identity/application/ports"
	"quizzivy/internal/modules/identity/application/token"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/opt"
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
	Principals ports.Principals
	Avatars    ports.ObjectStore
	Photos     ports.PhotoProcessor
	Log        *slog.Logger
}

// SetAvatars wires profile photos: the store that keeps them and the processor
// that makes them. Either nil leaves photos unavailable rather than
// half-configured; a failure the service survives is logged to logger.
func (s *Service) SetAvatars(store ports.ObjectStore, photos ports.PhotoProcessor, logger *slog.Logger) {
	if store == nil || photos == nil {
		return
	}
	s.Avatars, s.Photos, s.Log = store, photos, logger
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
	session, err := s.IssueSession(ctx, user, in.Origin())
	if errors.Is(err, domain.ErrAccountChanged) {
		user, err = s.Users.FindUserByID(ctx, user.ID)
		if err != nil {
			return model.GoogleSignInResult{}, fmt.Errorf("reload signing-in user: %w", err)
		}
		if user.Disabled() {
			return model.GoogleSignInResult{}, domain.ErrAccountDisabled
		}
		session, err = s.IssueSession(ctx, user, in.Origin())
	}
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
	return &Service{Users: users, Tokens: tokens, RefreshTTL: refreshTTL, Now: time.Now, Principals: noPrincipals{}}
}

// SetClock replaces the time source. Tests only.
func (s *Service) SetClock(now func() time.Time) { s.Now = now }

func (s *Service) IssueSession(ctx context.Context, user domain.User, origin Origin) (model.Session, error) {
	principal, err := s.Principals.Resolve(ctx, user.ID)
	if err != nil {
		return model.Session{}, fmt.Errorf("resolve permissions: %w", err)
	}
	access, err := s.Tokens.Issue(user.ID, user.Role, user.SessionEpoch)
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
	rec.UserAgent = opt.String(origin.UserAgent)
	rec.IP = opt.String(origin.IP)
	rec.GeoLabel = opt.String(origin.GeoLabel)
	if err := s.Users.CreateRefreshToken(ctx, rec, domain.SessionBasis{Epoch: user.SessionEpoch, PasswordHash: user.PasswordHash}); err != nil {
		return model.Session{}, fmt.Errorf("store refresh token: %w", err)
	}

	return model.Session{
		AccessToken:  access,
		ExpiresIn:    int(s.Tokens.TTL().Seconds()),
		RefreshToken: refresh,
		User:         user,
		Permissions:  principal.Permissions,
	}, nil
}
