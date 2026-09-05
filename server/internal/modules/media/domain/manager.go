package domain

import (
	"fmt"
)

// AssetManager is the acceptance policy for uploads: what may be stored and how long it may run.
type AssetManager struct{}

// CheckDuration refuses audio longer than the practice's listening tasks ever are.
func (AssetManager) CheckDuration(durationMs *int) error {
	if durationMs != nil && *durationMs > MaxDurationMs {
		return fmt.Errorf("%w: %d ms", ErrTooLong, *durationMs)
	}
	return nil
}

var Assets AssetManager
