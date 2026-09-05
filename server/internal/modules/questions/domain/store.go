package domain

import (
	"time"
)

// WriteInput is a create or an update, depending on whether ID is set.
type WriteInput struct {
	ID             string // empty to create
	Input          Input
	MediaAssetKind *string
	ActorID        string
	Now            time.Time
	IP             string
	UserAgent      string
}
