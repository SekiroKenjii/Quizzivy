package model

import (
	"time"
)

// SignedURLResult is a minted capability and the moment it stops working.
type SignedURLResult struct {
	URL       string
	ExpiresAt time.Time
}
