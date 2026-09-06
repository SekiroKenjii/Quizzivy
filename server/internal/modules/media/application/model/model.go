package model

import (
	"time"
)

type

// SignedURLResult is a minted capability and the moment it stops working.
SignedURLResult struct {
	URL       string
	ExpiresAt time.Time
}
