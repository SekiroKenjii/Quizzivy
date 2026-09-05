package domain

import (
	"errors"
	"time"
)

// §11.1's limits. They are also CHECKs on media_assets -- these produce the
// Vietnamese message, the constraints keep the rule true if a code path is
// added later that skips this one.
const (
	MaxBytes      int64 = 10 * 1024 * 1024
	MaxDurationMs       = 5 * 60 * 1000
)

var (
	ErrTooLarge = errors.New("media: file is larger than the limit")
	ErrTooLong  = errors.New("media: audio is longer than the limit")
)

type Kind string

const (
	KindAudio Kind = "audio"
	KindImage Kind = "image"
)

// Asset is a stored media row.
type Asset struct {
	ID               string
	Kind             Kind
	StorageKey       string
	MimeType         string
	Bytes            int64
	DurationMs       *int
	OriginalFilename string
	ChecksumSHA256   []byte
	CreatedAt        time.Time
	UsageCount       int
	// The versions behind UsageCount, so a blocked delete can name them.
	UsedIn []TestRef
	URL    string
}
