package ports

import (
	"context"

	notificationscommand "quizzivy/internal/modules/notifications/application/command"
	"quizzivy/internal/shared/cqrs"
)

// Notifier tells a user that something happened, in the app: the notifications module's Notify command.
type Notifier = cqrs.CommandHandler[notificationscommand.Notify, cqrs.Nothing]

// Zones resolves a user's IANA calendar zone: the identity module's effective
// zone, which is the profile preference or the documented default.
type Zones interface {
	ZoneOf(ctx context.Context, userID string) (string, error)
}

// Avatars signs the URL of a profile photo from the key a user row holds, and
// answers an empty URL when there is none to show. The identity module owns the
// object store, so an adapter in the composition root stands between the two.
type Avatars interface {
	AvatarURL(ctx context.Context, key string) (string, error)
}
