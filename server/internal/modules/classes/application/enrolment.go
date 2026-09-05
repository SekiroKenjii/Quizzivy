package application

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/modules/classes/domain/joincode"
	"time"
)

type Enrolment struct {
	repo domain.Repository
	now  func() time.Time
}

func NewEnrolment(repo domain.Repository) *Enrolment {
	return &Enrolment{repo: repo, now: time.Now}
}

// SetClock replaces the time source. Tests only.
func (s *Enrolment) SetClock(now func() time.Time) { s.now = now }

// Rotate issues a new join code, revoking any existing one.
func (s *Enrolment) Rotate(ctx context.Context, req domain.RotateRequest) (domain.Rotated, error) {
	code, err := joincode.Generate()
	if err != nil {
		return domain.Rotated{}, err
	}

	days := domain.DefaultExpiryDays
	if req.ExpiresInDays != nil {
		days = *req.ExpiresInDays
	}
	maxUses := domain.DefaultMaxUses
	if req.MaxUses != nil {
		maxUses = *req.MaxUses
	}

	now := s.now()
	issued, err := s.repo.Rotate(ctx, domain.RotateInput{
		ClassID:     req.ClassID,
		ActorUserID: req.ActorUserID,
		CodeHash:    joincode.Hash(code),
		Hint:        joincode.Hint(code),
		ExpiresAt:   now.AddDate(0, 0, days),
		MaxUses:     &maxUses,
		Now:         now,
		IP:          optional(req.IP),
		UserAgent:   optional(req.UserAgent),
	})
	if err != nil {
		return domain.Rotated{}, err
	}

	return domain.Rotated{
		Code:      joincode.Format(code),
		Hint:      issued.Hint,
		ExpiresAt: issued.ExpiresAt,
		MaxUses:   issued.MaxUses,
	}, nil
}

// Revoke ends the active code and closes self-join (§6.4).
func (s *Enrolment) Revoke(ctx context.Context, req domain.RevokeRequest) error {
	return s.repo.Revoke(ctx, domain.RevokeInput{
		ClassID:     req.ClassID,
		ActorUserID: req.ActorUserID,
		Now:         s.now(),
		IP:          optional(req.IP),
		UserAgent:   optional(req.UserAgent),
	})
}

// ActiveCode returns the live code's metadata, or nil.
func (s *Enrolment) ActiveCode(ctx context.Context, classID string) (*domain.IssuedCode, error) {
	c, err := s.repo.ActiveCode(ctx, classID)
	if err != nil {
		return nil, fmt.Errorf("active code for class %s: %w", classID, err)
	}
	return c, nil
}

// EnrolNewMember creates an account and enrols it (§6.3). The signup path.
func (s *Enrolment) EnrolNewMember(ctx context.Context, m domain.NewMember, rawCode string, meta domain.Meta) (domain.EnrolResult, error) {
	return s.repo.Enrol(ctx, domain.EnrolInput{
		RawCode:   rawCode,
		NewMember: &m,
		Now:       s.now(),
		IP:        optional(meta.IP),
		UserAgent: optional(meta.UserAgent),
	})
}

// EnrolExisting enrols a student who is already signed in (§6.2).
func (s *Enrolment) EnrolExisting(ctx context.Context, userID, rawCode string, meta domain.Meta) (domain.EnrolResult, error) {
	return s.repo.Enrol(ctx, domain.EnrolInput{
		RawCode:        rawCode,
		ExistingUserID: userID,
		Now:            s.now(),
		IP:             optional(meta.IP),
		UserAgent:      optional(meta.UserAgent),
	})
}
