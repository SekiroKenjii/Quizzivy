package domain

import (
	"fmt"
	"strings"
	"unicode/utf8"
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

// CheckSize refuses a file over the limit of its kind: ErrImageTooLarge for
// an image, ErrTooLarge for audio.
func (AssetManager) CheckSize(kind Kind, bytes int64) error {
	if kind == KindImage && bytes > MaxImageBytes {
		return ErrImageTooLarge
	}
	if bytes > MaxAudioBytes {
		return ErrTooLarge
	}
	return nil
}

// CheckQuota refuses with ErrQuotaExceeded a library that already holds
// held bytes and would pass quota with add more.
func (AssetManager) CheckQuota(held, add, quota int64) error {
	if held+add > quota {
		return ErrQuotaExceeded
	}
	return nil
}

// CheckPlayLimit refuses a default play limit outside 0 to MaxDefaultPlays,
// and any limit on an asset that is not audio. A nil limit is always allowed.
func (AssetManager) CheckPlayLimit(kind Kind, plays *int) error {
	if plays == nil {
		return nil
	}
	if *plays < 0 || *plays > MaxDefaultPlays {
		return fmt.Errorf("%w: %d", ErrInvalidPlayLimit, *plays)
	}
	if kind != KindAudio {
		return ErrPlayLimitOnImage
	}
	return nil
}

// DisplayName trims a name for storage and refuses one that is then blank or
// longer than MaxDisplayNameLength characters.
func (AssetManager) DisplayName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > MaxDisplayNameLength {
		return "", ErrInvalidName
	}
	return name, nil
}

var Assets AssetManager
