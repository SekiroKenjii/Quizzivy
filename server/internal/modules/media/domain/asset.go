// Package domain is the media context: the Asset aggregate (an uploaded file
// in object storage), its Kind, the limits AssetManager enforces, and who
// references an asset so it cannot be deleted from under a version.
package domain

import (
	"time"
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

type Kind string

const (
	KindAudio Kind = "audio"
	KindImage Kind = "image"
)

// §11.1's limits. They are also CHECKs on media_assets -- these produce the
// Vietnamese message, the constraints keep the rule true if a code path is
// added later that skips this one.
const (
	MaxBytes      int64 = 10 * 1024 * 1024
	MaxDurationMs       = 5 * 60 * 1000
)
