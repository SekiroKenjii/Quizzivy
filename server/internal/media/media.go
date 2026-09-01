package media

import (
	"errors"
	"fmt"
	"io"
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
	URL              string
}

// imageTypes is §11.1's image allowlist, matched on magic bytes.
//
// Extensions and Content-Type are not consulted, for images as for audio: both
// come from the uploader, and the question is what the file is.
func sniffImage(head []byte) string {
	switch {
	case len(head) >= 8 && string(head[0:8]) == "\x89PNG\r\n\x1a\n":
		return "image/png"
	case len(head) >= 3 && head[0] == 0xFF && head[1] == 0xD8 && head[2] == 0xFF:
		return "image/jpeg"
	case len(head) >= 12 && string(head[0:4]) == "RIFF" && string(head[8:12]) == "WEBP":
		return "image/webp"
	}
	return ""
}

// boundedCopy copies at most limit+1 bytes, so exceeding the limit is
// detectable without ever holding limit+n of an attacker's choosing.
//
// The +1 matters: copying exactly `limit` cannot distinguish a file at the
// limit from one over it, and "10.0 MB exactly" is a file we accept.
func boundedCopy(dst io.Writer, src io.Reader, limit int64) (int64, error) {
	n, err := io.Copy(dst, io.LimitReader(src, limit+1))
	if err != nil {
		return n, fmt.Errorf("media: reading upload: %w", err)
	}
	if n > limit {
		return n, ErrTooLarge
	}
	return n, nil
}
