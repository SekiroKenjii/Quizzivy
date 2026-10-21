package ports

import "context"

// Zones resolves the IANA calendar zone a viewer's days are counted in.
type Zones interface {
	ZoneOf(ctx context.Context, userID string) (string, error)
}

// Avatars signs the URL of a profile photo from the key a user row holds, and
// answers an empty URL when there is none to show.
type Avatars interface {
	AvatarURL(ctx context.Context, key string) (string, error)
}
