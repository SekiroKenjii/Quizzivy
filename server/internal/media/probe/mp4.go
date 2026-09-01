package probe

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

// maxAtoms bounds the walk. A crafted file can nest atoms or declare a size of
// zero; neither should turn an upload into an infinite loop.
const maxAtoms = 4096

// mp4Duration finds moov/mvhd and computes duration / timescale.
//
// Only two atoms matter, but they can be anywhere: `moov` sits after `mdat` in
// a file written straight to disk, and before it in one that has been prepared
// for streaming. So the top level is walked rather than assumed.
func mp4Duration(r io.ReaderAt, size int64) (int, error) {
	moovOffset, moovSize, err := findAtom(r, 0, size, "moov")
	if err != nil {
		return 0, fmt.Errorf("mp4: %w", err)
	}
	mvhdOffset, _, err := findAtom(r, moovOffset, moovOffset+moovSize, "mvhd")
	if err != nil {
		return 0, fmt.Errorf("mp4: %w", err)
	}
	head := make([]byte, 4)
	if _, err := r.ReadAt(head, mvhdOffset); err != nil {
		return 0, errors.New("mp4: unreadable mvhd")
	}

	switch head[0] {
	case 0:
		body := make([]byte, 12) // created, modified, timescale
		if _, err := r.ReadAt(body, mvhdOffset+4); err != nil {
			return 0, errors.New("mp4: short mvhd (v0)")
		}
		timescale := binary.BigEndian.Uint32(body[8:12])
		durationRaw := make([]byte, 4)
		if _, err := r.ReadAt(durationRaw, mvhdOffset+16); err != nil {
			return 0, errors.New("mp4: short mvhd duration (v0)")
		}
		return scaled(uint64(binary.BigEndian.Uint32(durationRaw)), uint64(timescale))

	case 1:
		body := make([]byte, 20) // created(8), modified(8), timescale(4)
		if _, err := r.ReadAt(body, mvhdOffset+4); err != nil {
			return 0, errors.New("mp4: short mvhd (v1)")
		}
		timescale := binary.BigEndian.Uint32(body[16:20])
		durationRaw := make([]byte, 8)
		if _, err := r.ReadAt(durationRaw, mvhdOffset+24); err != nil {
			return 0, errors.New("mp4: short mvhd duration (v1)")
		}
		return scaled(binary.BigEndian.Uint64(durationRaw), uint64(timescale))

	default:
		return 0, fmt.Errorf("mp4: unsupported mvhd version %d", head[0])
	}
}

func scaled(duration, timescale uint64) (int, error) {
	if timescale == 0 {
		return 0, errors.New("mp4: zero timescale")
	}
	if duration == 0 || duration == 0xFFFFFFFF || duration == 0xFFFFFFFFFFFFFFFF {
		return 0, errors.New("mp4: duration is not set")
	}
	if duration > math.MaxUint64/1000 {
		return 0, errors.New("mp4: implausible duration")
	}
	ms := duration * 1000 / timescale
	if ms > uint64(1<<31-1) {
		return 0, errors.New("mp4: implausible duration")
	}
	return int(ms), nil
}

// findAtom scans one level for a box, descending into `moov` when looking for
// something inside it. Returns the offset of the atom's BODY and its length.
func findAtom(r io.ReaderAt, from, until int64, want string) (int64, int64, error) {
	header := make([]byte, 8)
	offset := from
	for seen := 0; offset+8 <= until && seen < maxAtoms; seen++ {
		if _, err := r.ReadAt(header, offset); err != nil {
			return 0, 0, errors.New("unreadable atom header")
		}
		size := int64(binary.BigEndian.Uint32(header[0:4]))
		name := string(header[4:8])
		bodyAt := offset + 8

		switch size {
		case 0:
			// "To the end of file" -- legal for the last atom.
			size = until - offset
		case 1:
			// 64-bit size follows the header.
			wide := make([]byte, 8)
			if _, err := r.ReadAt(wide, offset+8); err != nil {
				return 0, 0, errors.New("unreadable 64-bit atom size")
			}
			size = int64(binary.BigEndian.Uint64(wide))
			bodyAt = offset + 16
		}
		if size < 8 || offset+size > until {
			return 0, 0, fmt.Errorf("atom %q has an impossible size", name)
		}
		if name == want {
			return bodyAt, size - (bodyAt - offset), nil
		}
		offset += size
	}
	return 0, 0, fmt.Errorf("no %q atom", want)
}
