package ports

import "context"

// GroupMediaKinds resolves attachment kinds without trusting a caller's claimed asset type.
type GroupMediaKinds interface {
	Kind(context.Context, string) (string, error)
}
