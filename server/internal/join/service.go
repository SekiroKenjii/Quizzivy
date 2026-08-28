package join

import (
	"context"
	"fmt"
	"time"
)

// Defaults from §6.1 and O-06. Expiry is the spec's; the use cap is the
// deliberate change -- §6.1 defaults to unlimited, which means a forwarded code
// works until it expires, and forwarding rather than guessing is the realistic
// threat (R-02).
const (
	DefaultExpiryDays = 30
	DefaultMaxUses    = 40
)

type Service struct {
	store *Store
	now   func() time.Time
}

func NewService(store *Store) *Service {
	return &Service{store: store, now: time.Now}
}

// SetClock replaces the time source. Tests only.
func (s *Service) SetClock(now func() time.Time) { s.now = now }

type RotateRequest struct {
	ClassID       string
	ActorUserID   string
	ExpiresInDays *int
	MaxUses       *int
	IP            string
	UserAgent     string
}

// Rotated is the ONE time the plaintext exists outside the caller's browser.
type Rotated struct {
	Code      string // grouped XXXX-XXXX, for display
	Hint      string
	ExpiresAt time.Time
	MaxUses   *int
}

// Rotate issues a new join code, revoking any existing one.
func (s *Service) Rotate(ctx context.Context, req RotateRequest) (Rotated, error) {
	code, err := Generate()
	if err != nil {
		return Rotated{}, err
	}

	days := DefaultExpiryDays
	if req.ExpiresInDays != nil {
		days = *req.ExpiresInDays
	}
	maxUses := DefaultMaxUses
	if req.MaxUses != nil {
		maxUses = *req.MaxUses
	}

	now := s.now()
	issued, err := s.store.Rotate(ctx, RotateInput{
		ClassID:     req.ClassID,
		ActorUserID: req.ActorUserID,
		CodeHash:    Hash(code),
		Hint:        Hint(code),
		ExpiresAt:   now.AddDate(0, 0, days),
		MaxUses:     &maxUses,
		Now:         now,
		IP:          optional(req.IP),
		UserAgent:   optional(req.UserAgent),
	})
	if err != nil {
		return Rotated{}, err
	}

	return Rotated{
		Code:      Format(code),
		Hint:      issued.Hint,
		ExpiresAt: issued.ExpiresAt,
		MaxUses:   issued.MaxUses,
	}, nil
}

type RevokeRequest struct {
	ClassID     string
	ActorUserID string
	IP          string
	UserAgent   string
}

// Revoke ends the active code and closes self-join (§6.4).
func (s *Service) Revoke(ctx context.Context, req RevokeRequest) error {
	return s.store.Revoke(ctx, RevokeInput{
		ClassID:     req.ClassID,
		ActorUserID: req.ActorUserID,
		Now:         s.now(),
		IP:          optional(req.IP),
		UserAgent:   optional(req.UserAgent),
	})
}

// ActiveCode returns the live code's metadata, or nil.
func (s *Service) ActiveCode(ctx context.Context, classID string) (*IssuedCode, error) {
	c, err := s.store.ActiveCode(ctx, classID)
	if err != nil {
		return nil, fmt.Errorf("active code for class %s: %w", classID, err)
	}
	return c, nil
}

func optional(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// Meta is the request context §6.5 requires on every enrolment audit row.
type Meta struct {
	IP        string
	UserAgent string
}

// EnrolNewMember creates an account and enrols it (§6.3). The signup path.
func (s *Service) EnrolNewMember(ctx context.Context, m NewMember, rawCode string, meta Meta) (EnrolResult, error) {
	return s.store.Enrol(ctx, EnrolInput{
		RawCode:   rawCode,
		NewMember: &m,
		Now:       s.now(),
		IP:        optional(meta.IP),
		UserAgent: optional(meta.UserAgent),
	})
}

// EnrolExisting enrols a student who is already signed in (§6.2).
func (s *Service) EnrolExisting(ctx context.Context, userID, rawCode string, meta Meta) (EnrolResult, error) {
	return s.store.Enrol(ctx, EnrolInput{
		RawCode:        rawCode,
		ExistingUserID: userID,
		Now:            s.now(),
		IP:             optional(meta.IP),
		UserAgent:      optional(meta.UserAgent),
	})
}
