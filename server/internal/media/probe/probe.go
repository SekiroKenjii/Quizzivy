package probe

import (
	"errors"
	"fmt"
	"io"
)

// ErrUnsupportedType is anything outside §11.1's allowlist.
var ErrUnsupportedType = errors.New("probe: unsupported media type")

// ErrUnmeasurable is a file that sniffs as audio but whose duration cannot be
// read.
//
// It is an ERROR rather than a null duration on purpose. media_assets CHECKs
// that an audio row has a duration, so storing one without would fail at the
// database with a 500 -- and "we could not read this file" is a better thing to
// tell a teacher than "something went wrong".
var ErrUnmeasurable = errors.New("probe: cannot determine duration")

// The audio MIME types this package identifies, per §11.1's allowlist.
const (
	MIMEMP3 = "audio/mpeg"
	MIMEMP4 = "audio/mp4"
)

// Audio identifies and measures an audio file.
//
// Identification is by MAGIC BYTES, never by extension or by the
// Content-Type header (§11.1). Both are attacker-supplied on an upload, and the
// question being answered is "what is this file", not "what does the uploader
// say it is".
func Audio(r io.ReaderAt, size int64) (mime string, durationMs int, err error) {
	if size <= 0 {
		return "", 0, fmt.Errorf("%w: empty file", ErrUnsupportedType)
	}

	switch sniff(r, size) {
	case MIMEMP3:
		ms, err := mp3Duration(r, size)
		if err != nil {
			return "", 0, fmt.Errorf("%w: %v", ErrUnmeasurable, err)
		}
		return MIMEMP3, ms, nil

	case MIMEMP4:
		ms, err := mp4Duration(r, size)
		if err != nil {
			return "", 0, fmt.Errorf("%w: %v", ErrUnmeasurable, err)
		}
		return MIMEMP4, ms, nil

	default:
		return "", 0, ErrUnsupportedType
	}
}

// sniff identifies the container from its leading bytes.
func sniff(r io.ReaderAt, _ int64) string {
	head := make([]byte, 16)
	n, err := r.ReadAt(head, 0)
	if err != nil && n < 12 {
		return ""
	}
	head = head[:n]
	if len(head) >= 12 && string(head[4:8]) == "ftyp" {
		switch string(head[8:12]) {
		case "M4A ", "M4B ", "mp42", "mp41", "isom", "iso2", "dash", "M4V ":
			return MIMEMP4
		default:
			return ""
		}
	}

	// An ID3v2 tag means mp3 in every practical case.
	if len(head) >= 3 && string(head[0:3]) == "ID3" {
		return MIMEMP3
	}
	if len(head) >= 4 {
		if _, err := parseFrameHeader(head[0:4]); err == nil {
			return MIMEMP3
		}
	}
	return ""
}
