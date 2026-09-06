package command

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/cqrs"
	"quizzivy/internal/shared/opt"
)

// Revoke ends the active code and closes self-join (§6.4).
type Revoke struct {
	Request domain.RevokeRequest
}

type RevokeHandler struct {
	*support.Enrolment
}

func (s RevokeHandler) Handle(ctx context.Context, cmd Revoke) (cqrs.Nothing, error) {
	return cqrs.Nothing{}, s.Repo.Revoke(ctx, domain.RevokeInput{
		ClassID:     cmd.Request.ClassID,
		ActorUserID: cmd.Request.ActorUserID,
		Now:         s.Now(),
		IP:          opt.String(cmd.Request.IP),
		UserAgent:   opt.String(cmd.Request.UserAgent),
	})
}
