package command

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/opt"
)

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

	now := s.Now()
	issued, err := s.Repo.Rotate(ctx, domain.RotateInput{
		ClassID:     cmd.Request.ClassID,
		ActorUserID: cmd.Request.ActorUserID,
		CodeHash:    domain.JoinCodes.Hash(code),
		Hint:        domain.JoinCodes.Hint(code),
		ExpiresAt:   now.AddDate(0, 0, days),
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
