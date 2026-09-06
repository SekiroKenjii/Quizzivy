package command

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/cqrs"
	"quizzivy/internal/shared/opt"
)

// UnlinkGoogle detaches the Google identity (§15).
type UnlinkGoogle struct {
	Actor actor.Actor
}

type UnlinkGoogleHandler struct {
	*support.Service
}

func (s UnlinkGoogleHandler) Handle(ctx context.Context, cmd UnlinkGoogle) (cqrs.Nothing, error) {
	user, err := s.Users.FindUserByID(ctx, cmd.Actor.ID)
	if err != nil {
		return cqrs.Nothing{}, err
	}
	if user.Disabled() {
		return cqrs.Nothing{}, domain.ErrAccountDisabled
	}
	if !user.HasPassword() {
		return cqrs.Nothing{}, domain.ErrLastLoginMethod
	}

	removed, err := s.Users.UnlinkIdentity(ctx, cmd.Actor.ID, "google")
	if err != nil {
		return cqrs.Nothing{}, err
	}
	if !removed {
		return cqrs.Nothing{}, nil
	}

	return cqrs.Nothing{}, s.Users.WriteAudit(ctx, audit.Entry{
		ActorUserID: &cmd.Actor.ID,
		Action:      "user.google_unlinked",
		Entity:      "user_identity",
		EntityID:    &cmd.Actor.ID,
		OccurredAt:  s.Now(),
		IP:          opt.String(cmd.Actor.IP),
		UserAgent:   opt.String(cmd.Actor.UserAgent),
	})
}
