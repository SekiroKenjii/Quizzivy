package support

import (
	"fmt"
	"io"
	"path"
	"quizzivy/internal/modules/media/domain"
	"strings"
	"time"

	"github.com/google/uuid"
)

// imageTypes is §11.1's image allowlist, matched on magic bytes.
//
// Extensions and Content-Type are not consulted, for images as for audio: both
// come from the uploader, and the question is what the file is.
func SniffImage(head []byte) string {
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

// BoundedCopy copies at most limit+1 bytes, so exceeding the limit is
// detectable without ever holding limit+n of an attacker's choosing.
//
// The +1 matters: copying exactly `limit` cannot distinguish a file at the
// limit from one over it, and "10.0 MB exactly" is a file we accept.
func BoundedCopy(dst io.Writer, src io.Reader, limit int64) (int64, error) {
	n, err := io.Copy(dst, io.LimitReader(src, limit+1))
	if err != nil {
		return n, fmt.Errorf("media: reading upload: %w", err)
	}
	if n > limit {
		return n, domain.ErrTooLarge
	}
	return n, nil
}

type UploadInput struct {
	Filename   string
	Body       io.Reader
	UploaderID string
	IP         string
	UserAgent  string
}

func ExtensionFor(mime string) string {
	switch mime {
	case "audio/mpeg":
		return ".mp3"
	case "audio/mp4", "audio/aac":
		return ".m4a"
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	}
	return ""
}

func SanitiseFilename(name string) string {
	name = path.Base(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"))
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == "/" {
		return "tệp-không-tên"
	}
	if r := []rune(name); len(r) > 200 {
		name = string(r[:200])
	}
	return name
}

// DefaultSignedURLTTL is §11.2's ten minutes. Short because the URL IS the
// capability: one that outlives its purpose cannot be revoked afterwards.
const DefaultSignedURLTTL = 10 * time.Minute

// NewAssetID mints the id up front, because the storage key contains it -- the
// object has to be written before the row exists, so the row cannot supply it.
func NewAssetID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", domain.ErrNoID
	}
	return id.String(), nil
}
