package command

import (
	"context"

	"quizzivy/internal/modules/organization/application/internal/support"
	"quizzivy/internal/modules/organization/domain"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/validation"
)

// UpdateSettings replaces the centre's settings with Settings, read at
// ExpectedRevision, for Actor. Settings is normalised and checked first; a
// logo it names that the stored document does not must be in the logo store.
// It fails with domain.ErrStale, or *domain.LockedOut when the sign-in policy
// would leave Actor or every active built-in Admin without a way in, and
// otherwise returns the document as stored, which this server serves at once.
// KeepLimits keeps the stored usage limits, for a client that sends none.
type UpdateSettings struct {
	Actor            actor.Actor
	Settings         domain.Settings
	ExpectedRevision int64
	KeepLimits       bool
}

type UpdateSettingsHandler struct {
	*support.Service
}

func (h UpdateSettingsHandler) Handle(ctx context.Context, cmd UpdateSettings) (domain.Settings, error) {
	next, err := cmd.Settings.Normalised()
	if err != nil {
		return domain.Settings{}, err
	}
	stored, err := h.Repo.Read(ctx)
	if err != nil {
		return domain.Settings{}, err
	}
	if cmd.KeepLimits {
		next.Limits = stored.Limits
	}
	if err := h.checkLogos(ctx, stored.Profile, next.Profile); err != nil {
		return domain.Settings{}, err
	}
	saved, err := h.Repo.Save(ctx, domain.Save{
		Settings: next, ExpectedRevision: cmd.ExpectedRevision, ActorID: cmd.Actor.ID,
		IP: cmd.Actor.IP, UserAgent: cmd.Actor.UserAgent, Now: h.Now(), GoogleConfigured: h.GoogleConfigured,
	})
	if err != nil {
		return domain.Settings{}, err
	}
	h.Snapshot.Remember(saved)
	return saved, nil
}

func (h UpdateSettingsHandler) checkLogos(ctx context.Context, stored, next domain.Profile) error {
	var refused validation.Error
	for _, v := range []domain.Variant{domain.Light, domain.Dark} {
		logo := next.Logo(v)
		if logo == nil || logo.Equal(stored.Logo(v)) {
			continue
		}
		field := "organization.logo." + string(v)
		if h.Logos == nil {
			refused.Add(field, domain.ReasonLogoStore)
			continue
		}
		held, err := h.Logos.Exists(ctx, support.LogoKey(*logo))
		if err != nil {
			return err
		}
		if !held {
			refused.Add(field, domain.ReasonLogoGone)
		}
	}
	if err := refused.OrNil(); err != nil {
		return err
	}
	return nil
}
