package command

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/opt"

	"github.com/google/uuid"
)

// Rotate issues a new join code, revoking any existing one. The code works
// through 23:59:59 on the day ExpiresInDays after today in the calendar zone of
// the actor's profile (domain.CodeExpiry).
type Rotate struct {
	Request domain.RotateRequest
}

type RotateHandler struct {
	*support.Enrolment
}

func (s RotateHandler) Handle(ctx context.Context, cmd Rotate) (domain.Rotated, error) {
	code, err := domain.JoinCodes.Generate()
	if err != nil {
		return domain.Rotated{}, err
	}

	days := domain.DefaultExpiryDays
	if cmd.Request.ExpiresInDays != nil {
		days = *cmd.Request.ExpiresInDays
	}
	maxUses := domain.DefaultMaxUses
	if cmd.Request.MaxUses != nil {
		maxUses = *cmd.Request.MaxUses
	}

	codeID, err := uuid.NewV7()
	if err != nil {
		return domain.Rotated{}, fmt.Errorf("join code id: %w", err)
	}
	sealed, err := s.Keys.Seal(cmd.Request.ClassID, codeID.String(), code)
	if err != nil {
		return domain.Rotated{}, err
	}

	now := s.Now()
	issued, err := s.Repo.Rotate(ctx, domain.RotateInput{
		ClassID:     cmd.Request.ClassID,
		ActorUserID: cmd.Request.ActorUserID,
		All:         cmd.Request.All,
		CodeID:      codeID.String(),
		CodeHash:    s.Keys.Hash(code),
		Ciphertext:  sealed,
		KeyID:       s.Keys.CurrentID(),
		Hint:        domain.JoinCodes.Hint(code),
		ExpiresAt:   domain.CodeExpiry(now, days, s.ZoneOf(ctx, cmd.Request.ActorUserID)),
		MaxUses:     &maxUses,
		Now:         now,
		IP:          opt.String(cmd.Request.IP),
		UserAgent:   opt.String(cmd.Request.UserAgent),
	})
	if err != nil {
		return domain.Rotated{}, err
	}

	return domain.Rotated{
		Code:      domain.JoinCodes.Format(code),
		Hint:      issued.Hint,
		ExpiresAt: issued.ExpiresAt,
		MaxUses:   issued.MaxUses,
	}, nil
}
