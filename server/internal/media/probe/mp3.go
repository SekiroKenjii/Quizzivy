package probe

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// MPEG audio frame tables. Index 0 is "free" and index 15 is "bad"; both are
// treated as invalid, which is also how a false sync is usually caught.
var (
	// [version][bitrateIndex], kbps. Layer III only -- §11.1 accepts mp3.
	bitrates = map[mpegVersion][16]int{
		mpeg1:  {0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0},
		mpeg2:  {0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},
		mpeg25: {0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},
	}
	sampleRates = map[mpegVersion][4]int{
		mpeg1:  {44100, 48000, 32000, 0},
		mpeg2:  {22050, 24000, 16000, 0},
		mpeg25: {11025, 12000, 8000, 0},
	}
	// Layer III samples per frame. MPEG2/2.5 halve it.
	samplesPerFrame = map[mpegVersion]int{mpeg1: 1152, mpeg2: 576, mpeg25: 576}
)

type mpegVersion int

const (
	mpegReserved mpegVersion = iota
	mpeg1
	mpeg2
	mpeg25
)

type frameHeader struct {
	version    mpegVersion
	bitrateKbs int
	sampleRate int
	channels   int
	frameLen   int
}

// parseFrameHeader decodes the four header bytes, or reports why they are not
// a frame. A "sync" that fails any of these checks is almost always a false
// positive inside audio data rather than a real frame.
func parseFrameHeader(b []byte) (frameHeader, error) {
	if len(b) < 4 {
		return frameHeader{}, errors.New("short header")
	}
	// 11 sync bits.
	if b[0] != 0xFF || b[1]&0xE0 != 0xE0 {
		return frameHeader{}, errors.New("no frame sync")
	}

	var version mpegVersion
	switch (b[1] >> 3) & 0x03 {
	case 0:
		version = mpeg25
	case 1:
		return frameHeader{}, errors.New("reserved MPEG version")
	case 2:
		version = mpeg2
	case 3:
		version = mpeg1
	}
	if (b[1]>>1)&0x03 != 0x01 {
		return frameHeader{}, errors.New("not layer III")
	}

	bitrateIndex := (b[2] >> 4) & 0x0F
	bitrate := bitrates[version][bitrateIndex]
	if bitrate == 0 {
		return frameHeader{}, fmt.Errorf("invalid bitrate index %d", bitrateIndex)
	}

	rateIndex := (b[2] >> 2) & 0x03
	sampleRate := sampleRates[version][rateIndex]
	if sampleRate == 0 {
		return frameHeader{}, fmt.Errorf("invalid sample-rate index %d", rateIndex)
	}

	padding := int((b[2] >> 1) & 0x01)
	channelMode := (b[3] >> 6) & 0x03
	channels := 2
	if channelMode == 3 { // single channel
		channels = 1
	}
	coefficient := 144
	if version != mpeg1 {
		coefficient = 72
	}
	frameLen := coefficient*bitrate*1000/sampleRate + padding
	if frameLen < 4 {
		return frameHeader{}, errors.New("degenerate frame length")
	}

	return frameHeader{
		version:    version,
		bitrateKbs: bitrate,
		sampleRate: sampleRate,
		channels:   channels,
		frameLen:   frameLen,
	}, nil
}

// skipID3v2 returns the offset of the first byte after an ID3v2 tag, or 0.
//
// The tag can be tens of kilobytes -- cover art -- so searching for a frame
// sync without skipping it first finds a false sync inside a JPEG surprisingly
// often.
func skipID3v2(r io.ReaderAt, size int64) (int64, error) {
	header := make([]byte, 10)
	if _, err := r.ReadAt(header, 0); err != nil {
		return 0, nil // Too small to have a tag; let the frame scan decide.
	}
	if string(header[0:3]) != "ID3" {
		return 0, nil
	}
	// A syncsafe integer: seven bits per byte, high bit always clear.
	for _, b := range header[6:10] {
		if b&0x80 != 0 {
			return 0, errors.New("mp3: malformed ID3v2 size")
		}
	}
	tagSize := int64(header[6])<<21 | int64(header[7])<<14 | int64(header[8])<<7 | int64(header[9])
	offset := 10 + tagSize
	// A footer is present when bit 4 of the flags is set.
	if header[5]&0x10 != 0 {
		offset += 10
	}
	if offset >= size {
		return 0, errors.New("mp3: ID3v2 tag claims to be larger than the file")
	}
	return offset, nil
}

// maxFrameScan bounds the work a crafted file can cause.
//
// Five minutes of the smallest legal frame (8 kbps, 8 kHz, 576 samples) is
// about 4,200 frames; §11.1 caps a file at five minutes and 10 MB, so anything
// beyond this is not a file we would store even if we finished measuring it.
const maxFrameScan = 200_000

// mp3Duration measures an MPEG audio stream.
//
// Prefers a Xing/Info or VBRI frame count, because that is the only accurate
// answer for a VBR file short of decoding it. Falls back to walking frames --
// which is exact for CBR and for VBR without a header, at the cost of touching
// every frame header in the file.
func mp3Duration(r io.ReaderAt, size int64) (int, error) {
	start, err := skipID3v2(r, size)
	if err != nil {
		return 0, err
	}

	offset, first, err := findFirstFrame(r, size, start)
	if err != nil {
		return 0, err
	}
	if frames, declaredBytes, ok := vbrFrameCount(r, offset, first); ok {
		if headerAgreesWithFile(frames, declaredBytes, first, size-offset) {
			return durationMs(frames, first), nil
		}
	}
	frames := 0
	header := make([]byte, 4)
	overran := false
	for offset < size && frames < maxFrameScan {
		if _, err := r.ReadAt(header, offset); err != nil {
			break
		}
		f, err := parseFrameHeader(header)
		if err != nil {
			next, resyncErr := resync(r, size, offset+1)
			if resyncErr != nil {
				break
			}
			offset = next
			continue
		}
		frames++
		offset += int64(f.frameLen)
		if offset > size {
			overran = true
		}
	}

	if overran {
		return 0, errors.New("mp3: stream ends inside a frame; the file looks truncated")
	}

	if frames == 0 {
		return 0, errors.New("mp3: no audio frames found")
	}
	if frames >= maxFrameScan {
		return 0, fmt.Errorf("mp3: gave up after %d frames", maxFrameScan)
	}
	return durationMs(frames, first), nil
}

func durationMs(frames int, f frameHeader) int {
	return int(int64(frames) * int64(samplesPerFrame[f.version]) * 1000 / int64(f.sampleRate))
}

// findFirstFrame locates the first valid frame at or after `from`.
func findFirstFrame(r io.ReaderAt, size, from int64) (int64, frameHeader, error) {
	offset, err := resync(r, size, from)
	if err != nil {
		return 0, frameHeader{}, err
	}
	header := make([]byte, 4)
	if _, err := r.ReadAt(header, offset); err != nil {
		return 0, frameHeader{}, errors.New("mp3: unreadable first frame")
	}
	f, err := parseFrameHeader(header)
	if err != nil {
		return 0, frameHeader{}, fmt.Errorf("mp3: %w", err)
	}
	return offset, f, nil
}

// maxResyncScan bounds the search for a frame sync. Generous enough for the
// padding real encoders leave, small enough that a non-MP3 file that sniffed
// wrong is rejected quickly.
const maxResyncScan = 64 * 1024

func resync(r io.ReaderAt, size, from int64) (int64, error) {
	buf := make([]byte, 4)
	limit := from + maxResyncScan
	for offset := from; offset+4 <= size && offset < limit; offset++ {
		if _, err := r.ReadAt(buf, offset); err != nil {
			return 0, errors.New("mp3: unreadable while looking for a frame")
		}
		if _, err := parseFrameHeader(buf); err == nil {
			return offset, nil
		}
	}
	return 0, errors.New("mp3: no frame sync found")
}

// vbrFrameCount reads a Xing/Info or VBRI header out of the first frame.
func vbrFrameCount(r io.ReaderAt, frameOffset int64, f frameHeader) (int, int64, bool) {
	sideInfo := int64(32) // MPEG1 stereo
	switch {
	case f.version == mpeg1 && f.channels == 1:
		sideInfo = 17
	case f.version != mpeg1 && f.channels == 1:
		sideInfo = 9
	case f.version != mpeg1:
		sideInfo = 17
	}
	if n, declared, ok := xingFrameCount(r, frameOffset+4+sideInfo); ok {
		return n, declared, true
	}
	return vbriFrameCount(r, frameOffset+4+32)
}

// readUint32 reads a big-endian uint32, reporting whether it could. Every
// header field below is one of these, and threading the read failure through as
// a bool is what keeps the two parsers flat.
func readUint32(r io.ReaderAt, at int64) (uint32, bool) {
	buf := make([]byte, 4)
	if _, err := r.ReadAt(buf, at); err != nil {
		return 0, false
	}
	return binary.BigEndian.Uint32(buf), true
}

// plausibleFrameCount rejects the two counts that cannot be real before they
// reach any arithmetic: zero, and more frames than the walk would ever scan.
func plausibleFrameCount(n uint32) (int, bool) {
	count := int(n)
	return count, count > 0 && count < maxFrameScan
}

// xingFrameCount reads a Xing/Info header at `at`, the offset of the tag.
func xingFrameCount(r io.ReaderAt, at int64) (int, int64, bool) {
	tag := make([]byte, 4)
	if _, err := r.ReadAt(tag, at); err != nil {
		return 0, 0, false
	}
	if s := string(tag); s != "Xing" && s != "Info" {
		return 0, 0, false
	}

	bits, ok := readUint32(r, at+4)
	if !ok || bits&xingHasFrames == 0 {
		return 0, 0, false
	}
	raw, ok := readUint32(r, at+8)
	if !ok {
		return 0, 0, false
	}
	count, ok := plausibleFrameCount(raw)
	if !ok {
		return 0, 0, false
	}
	var declared int64
	if bits&xingHasBytes != 0 {
		if b, ok := readUint32(r, at+12); ok {
			declared = int64(b)
		}
	}
	return count, declared, true
}

// vbriFrameCount reads a VBRI header at `at`, the offset of the tag. Its fields
// are at fixed offsets rather than flag-dependent: stream size at 10, frame
// count at 14.
func vbriFrameCount(r io.ReaderAt, at int64) (int, int64, bool) {
	tag := make([]byte, 4)
	if _, err := r.ReadAt(tag, at); err != nil || string(tag) != "VBRI" {
		return 0, 0, false
	}
	raw, ok := readUint32(r, at+14)
	if !ok {
		return 0, 0, false
	}
	count, ok := plausibleFrameCount(raw)
	if !ok {
		return 0, 0, false
	}

	var declared int64
	if b, ok := readUint32(r, at+10); ok {
		declared = int64(b)
	}
	return count, declared, true
}

// Xing flag bits. The optional fields are packed in this order, so an offset
// into them is only correct once the preceding flags are known.
const (
	xingHasFrames = 1 << 0
	xingHasBytes  = 1 << 1
)

// mp3BitrateBounds are the slowest and fastest bitrates any MPEG audio frame
// can declare, in bits per second. They bound how much audio a given number of
// bytes can possibly hold, whatever the encoder did in between.
const (
	minBitrateBps = 8_000
	maxBitrateBps = 320_000
)

// headerAgreesWithFile reports whether a declared frame count can be true of the
// bytes present.
//
// The count is four attacker-chosen bytes, so a file could claim a short
// duration over long audio and bypass the length limit; a truncated upload also
// keeps the header describing the whole file. The caller falls back to the frame
// walk when this fails, which is exact and detects truncation.
func headerAgreesWithFile(frames int, declaredBytes int64, f frameHeader, audioBytes int64) bool {
	if audioBytes <= 0 {
		return false
	}
	if declaredBytes > 0 && audioBytes < declaredBytes-512 {
		return false
	}
	ms := int64(durationMs(frames, f))
	if ms <= 0 {
		return false
	}
	shortestPossibleMs := audioBytes * 8 * 1000 / maxBitrateBps
	longestPossibleMs := audioBytes * 8 * 1000 / minBitrateBps
	return ms >= shortestPossibleMs && ms <= longestPossibleMs
}
