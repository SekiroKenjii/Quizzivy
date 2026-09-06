package command

import (
	"context"
	"crypto/sha256"
	"fmt"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/application/model"
	"quizzivy/internal/modules/identity/domain"
)

// Refresh rotates a refresh token (§5.2).
type Refresh struct {
	Token     string
	UserAgent string
	IP        string
}

type RefreshHandler struct {
	*support.Service
}

func (s RefreshHandler) Handle(ctx context.Context, cmd Refresh) (model.RefreshResult, error) {
	if cmd.Token == "" {
		return model.RefreshResult{}, domain.ErrRefreshRejected
	}
	presented := sha256.Sum256([]byte(cmd.Token))

	successor, successorHash, err := support.NewRefreshToken()
	if err != nil {
		return model.RefreshResult{}, err
	}

	now := s.Now()
	next := domain.RefreshTokenRecord{
		TokenHash: successorHash,
		IssuedAt:  now,
		ExpiresAt: now.Add(s.RefreshTTL),
	}
	if cmd.UserAgent != "" {
		next.UserAgent = &cmd.UserAgent
	}
	if cmd.IP != "" {
		next.IP = &cmd.IP
	}

	res, err := s.Users.Rotate(ctx, presented[:], next, now)
	if err != nil {
		return model.RefreshResult{}, fmt.Errorf("rotate refresh token: %w", err)
	}
	switch res.Outcome {
	case domain.RotateOK:
	case domain.RotateReused:

		return model.RefreshResult{}, domain.ErrRefreshReused
	default:
		return model.RefreshResult{}, domain.ErrRefreshRejected
	}

	access, err := s.Tokens.Issue(res.User.ID, res.User.Role)
	if err != nil {
		return model.RefreshResult{}, fmt.Errorf("issue access token: %w", err)
	}

	return model.RefreshResult{
		AccessToken:  access,
		ExpiresIn:    int(s.Tokens.TTL().Seconds()),
		RefreshToken: successor,
		User:         res.User,
	}, nil
}
