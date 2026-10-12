package ports

import "context"

// Zones resolves the caller's IANA calendar zone.
type Zones interface {
	ZoneOf(ctx context.Context, userID string) (string, error)
}
