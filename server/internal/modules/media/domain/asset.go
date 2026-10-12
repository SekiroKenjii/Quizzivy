// Package domain is the media context: the Asset aggregate (an uploaded file
// in object storage), its Kind, the limits AssetManager enforces, and who
// references an asset so it cannot be deleted from under a version.
package domain

import (
	"time"
)

// Asset is a stored media row. DisplayName is the name the library shows: the
// stored one, or OriginalFilename while none was set. DefaultMaxPlays is nil
// when no limit was set and 0 for unlimited. Width and Height are an image's
// pixels, both nil when unknown.
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
	DisplayName      string
	DefaultMaxPlays  *int
	Width            *int
	Height           *int
	QuestionCount    int
	UsageCount       int
	UsedIn           []TestRef
	URL              string
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
	MaxAudioBytes int64 = 50 << 20
	MaxImageBytes int64 = 10 << 20
	MaxDurationMs       = 5 * 60 * 1000
)

// MaxDisplayNameLength is the longest name the library stores, in characters.
const MaxDisplayNameLength = 200

// MaxDefaultPlays is the highest play limit an asset can carry; 0 is unlimited.
const MaxDefaultPlays = 3

// DefaultOwnerQuotaBytes is what one owner's library may hold when the
// deployment sets no quota, the default of MEDIA_OWNER_QUOTA_MIB.
const DefaultOwnerQuotaBytes int64 = 5120 << 20

// Facets counts a library's files by tab for one search.
type Facets struct {
	All    int
	Audio  int
	Image  int
	Unused int
}

// Usage is the bytes one library holds by kind, beside the quota it is
// measured against.
type Usage struct {
	AudioBytes int64
	ImageBytes int64
	QuotaBytes int64
}
