package probe_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"quizzivy/internal/media/probe"
)

// §11.1: identify by magic bytes, never by extension or Content-Type, and
// measure without shelling out to ffprobe.

func open(t *testing.T, name string) (*bytes.Reader, int64) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return bytes.NewReader(data), int64(len(data))
}

func TestDurationsAreWithinOnePercent(t *testing.T) {
	// 383 frames x 1152 samples / 44100 Hz = 10005.8 ms.
	const mp3TenSeconds = 10005

	for _, tc := range []struct {
		file   string
		mime   string
		wantMs int
		why    string
	}{
		{"cbr-128k.mp3", "audio/mpeg", mp3TenSeconds,
			"constant bitrate: every frame the same length"},
		{"vbr-xing.mp3", "audio/mpeg", mp3TenSeconds,
			"a Xing frame count, which is what a player trusts"},
		{"vbr-no-xing.mp3", "audio/mpeg", mp3TenSeconds,
			"no Xing header: only walking every frame gets this right (R-08)"},
		{"id3v2-large.mp3", "audio/mpeg", mp3TenSeconds,
			"a 32 KB tag of 0xFF, a false frame sync on every byte, which must be skipped"},
		{"mvhd-v0.m4a", "audio/mp4", 10_000,
			"32-bit mvhd, and moov placed after mdat"},
		{"mvhd-v1.m4a", "audio/mp4", 10_000,
			"64-bit mvhd, the version a long file needs"},
	} {
		t.Run(tc.file, func(t *testing.T) {
			r, size := open(t, tc.file)
			mime, ms, err := probe.Audio(r, size)
			if err != nil {
				t.Fatalf("%s (%s): %v", tc.file, tc.why, err)
			}
			if mime != tc.mime {
				t.Errorf("mime = %q, want %q", mime, tc.mime)
			}
			tolerance := tc.wantMs / 100
			if diff := ms - tc.wantMs; diff > tolerance || diff < -tolerance {
				t.Errorf("duration = %d ms, want %d ± %d (%s)", ms, tc.wantMs, tolerance, tc.why)
			}
		})
	}
}

func TestTheTwoRejectsAreRejected(t *testing.T) {
	t.Run("a WAV renamed to .mp3", func(t *testing.T) {
		r, size := open(t, "wav-renamed.mp3")
		_, _, err := probe.Audio(r, size)
		if !errors.Is(err, probe.ErrUnsupportedType) {
			t.Fatalf("error = %v, want ErrUnsupportedType", err)
		}
	})

	t.Run("a truncated MP3", func(t *testing.T) {
		r, size := open(t, "truncated.mp3")
		_, _, err := probe.Audio(r, size)
		if !errors.Is(err, probe.ErrUnmeasurable) {
			t.Fatalf("error = %v, want ErrUnmeasurable", err)
		}
	})
}

func TestNothingOutsideTheAllowlistIsAccepted(t *testing.T) {
	for name, data := range map[string][]byte{
		"empty":            {},
		"plain text":       []byte("xin chào, đây không phải là tệp âm thanh"),
		"a PNG":            append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 64)...),
		"an OGG container": append([]byte("OggS"), make([]byte, 64)...),
		"a FLAC stream":    append([]byte("fLaC"), make([]byte, 64)...),
		// ISO-BMFF, but a video profile rather than one §11.1 lists.
		"an unlisted ftyp brand": append([]byte{0, 0, 0, 0x18}, []byte("ftypqt  \x00\x00\x00\x00qt  ")...),
		// Two bytes that look like a sync but do not parse as a frame header.
		"a false frame sync": {0xFF, 0xFB, 0xFF, 0xFF, 0x00, 0x00},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := probe.Audio(bytes.NewReader(data), int64(len(data)))
			if err == nil {
				t.Fatal("accepted")
			}
		})
	}
}

func TestAnExtensionNeverDecidesTheType(t *testing.T) {
	m4a, size := open(t, "mvhd-v0.m4a")
	mime, _, err := probe.Audio(m4a, size)
	if err != nil {
		t.Fatal(err)
	}
	if mime != "audio/mp4" {
		t.Errorf("mime = %q; the bytes are ISO-BMFF whatever the file is called", mime)
	}
}

func TestACraftedFileCannotMakeTheProberLoop(t *testing.T) {
	t.Run("an mp4 atom of size zero", func(t *testing.T) {
		data := []byte{0, 0, 0, 0x18}
		data = append(data, []byte("ftypM4A \x00\x00\x00\x00M4A ")...)
		data = append(data, 0, 0, 0, 0) // size 0
		data = append(data, []byte("moov")...)
		data = append(data, make([]byte, 32)...)
		_, _, err := probe.Audio(bytes.NewReader(data), int64(len(data)))
		if err == nil {
			t.Error("accepted a file with a degenerate atom size")
		}
	})

	t.Run("a very long run of frame headers", func(t *testing.T) {
		frame := []byte{0xFF, 0xFB, 0x90, 0x00}
		data := make([]byte, 0, 4*1024*1024)
		for len(data) < 4*1024*1024 {
			f := make([]byte, 417)
			copy(f, frame)
			data = append(data, f...)
		}
		_, ms, err := probe.Audio(bytes.NewReader(data), int64(len(data)))
		if err == nil && ms <= 0 {
			t.Errorf("returned a nonsensical duration %d", ms)
		}
	})
}

// craftedMP4 builds a minimal ftyp + moov/mvhd with a version-1 (64-bit) mvhd,
// so a test can put an arbitrary declared duration in front of the prober.
// Separate from testdata/generate.go on purpose: that corpus is what real
// encoders produce, and this is what an uploader can produce.
func craftedMP4(timescale, duration uint64) []byte {
	atom := func(name string, body []byte) []byte {
		out := make([]byte, 8)
		binary.BigEndian.PutUint32(out[0:4], uint32(8+len(body)))
		copy(out[4:8], name)
		return append(out, body...)
	}
	ftyp := atom("ftyp", append([]byte("M4A "), append(
		[]byte{0, 0, 0, 0}, []byte("M4A mp42isom")...)...))

	body := make([]byte, 4+8+8+4+8+80)
	body[0] = 1 // version 1: 64-bit creation/modification times and duration
	binary.BigEndian.PutUint32(body[20:24], uint32(timescale))
	binary.BigEndian.PutUint64(body[24:32], duration)

	mdat := atom("mdat", make([]byte, 2048))
	return append(append(ftyp, mdat...), atom("moov", atom("mvhd", body))...)
}

// TestADeclaredDurationCannotOverflowIntoAPlausibleOne pins the arithmetic in
// `scaled`.
func TestADeclaredDurationCannotOverflowIntoAPlausibleOne(t *testing.T) {
	const (
		timescale = uint64(1000)
		// 73 million years of audio, chosen to wrap to exactly 30_000 ms.
		crafted = uint64(2305843009213723952)
	)
	d := crafted
	if wrapped := (d * 1000) / timescale; wrapped != 30_000 {
		t.Fatalf("premise broken: the crafted duration wraps to %d ms, not 30000", wrapped)
	}

	data := craftedMP4(timescale, crafted)
	_, ms, err := probe.Audio(bytes.NewReader(data), int64(len(data)))
	if err == nil {
		t.Errorf("a file declaring %d ticks at timescale %d was accepted as %d ms",
			crafted, timescale, ms)
	}
}

// TestAnHonestLongDurationIsStillRejectedAsTooLong keeps the new bound from
// being the only thing standing between a long file and acceptance: the plain
// too-long case must still be refused on its own merits.
func TestAnHonestLongDurationIsStillRejectedAsTooLong(t *testing.T) {
	// One hour at timescale 1000 -- no overflow anywhere, simply too long.
	data := craftedMP4(1000, 3_600_000)
	_, ms, err := probe.Audio(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("an hour-long file should probe cleanly and be refused later, got %v", err)
	}
	if ms != 3_600_000 {
		t.Errorf("duration = %d ms, want 3600000 -- the probe reports, §11.1 rejects", ms)
	}
}

// --------------------------------------------------- hostile Xing headers

// mp3WithXing builds a CBR stream of `realFrames` frames and writes a Xing
// header on the first one declaring `declaredFrames`. When the two disagree,
// the file is lying about its own length -- which is what an uploader controls.
func mp3WithXing(realFrames, declaredFrames int, declaredBytes uint32) []byte {
	const (
		frameLen            = 417
		sideInfoStereoMPEG1 = 32
	)
	stream := make([]byte, 0, frameLen*realFrames)
	for range realFrames {
		f := make([]byte, frameLen)
		// FF FB 90 00 -- MPEG1 Layer III, 128 kbps, 44.1 kHz, stereo.
		f[0], f[1], f[2], f[3] = 0xFF, 0xFB, 0x90, 0x00
		stream = append(stream, f...)
	}

	at := 4 + sideInfoStereoMPEG1
	copy(stream[at:], []byte("Xing"))
	flags := uint32(0x0001)
	if declaredBytes > 0 {
		flags |= 0x0002
	}
	binary.BigEndian.PutUint32(stream[at+4:], flags)
	binary.BigEndian.PutUint32(stream[at+8:], uint32(declaredFrames))
	if declaredBytes > 0 {
		binary.BigEndian.PutUint32(stream[at+12:], declaredBytes)
	}
	return stream
}

// TestAXingCountThatTheFileCannotHoldIsNotTrusted covers the §11.1 bypass.
//
// The declared frame count is four attacker-chosen bytes and used to be
// returned after nothing more than `0 < n < 200000`. A header claiming a short
// file over a long one made the probe report a short duration, media_assets
// accepted it against `duration_ms <= 300000`, and the five-minute limit became
// a number the upload chose for itself.
func TestAXingCountThatTheFileCannotHoldIsNotTrusted(t *testing.T) {
	// ~40 minutes of real audio, declaring 1000 frames (~26 seconds).
	const realFrames = 80_000
	data := mp3WithXing(realFrames, 1_000, 0)

	_, ms, err := probe.Audio(bytes.NewReader(data), int64(len(data)))
	if err == nil && ms < 60_000 {
		t.Errorf("trusted a header claiming %d ms for a file holding ~%d ms of audio",
			ms, realFrames*1152*1000/44100)
	}
}

// TestATruncatedVBRFileIsNotAcceptedAsComplete covers the other half.
//
// The frame walk detects truncation by noticing the final frame overruns the
// file, and that code never ran when a Xing header was present -- on VBR, the
// file type most likely to have one. A file cut in half still carries the
// header describing the whole, so it reported its original duration and passed
// as complete.
func TestATruncatedVBRFileIsNotAcceptedAsComplete(t *testing.T) {
	const (
		frames    = 400
		frameLen  = 417
		fullBytes = frames * frameLen
	)
	full := mp3WithXing(frames, frames, uint32(fullBytes))
	cut := full[:len(full)/2]

	_, ms, err := probe.Audio(bytes.NewReader(cut), int64(len(cut)))
	full_ms := frames * 1152 * 1000 / 44100
	if err == nil && ms >= full_ms {
		t.Errorf("a file cut to %d of %d bytes still reported its original %d ms",
			len(cut), len(full), ms)
	}
}

// TestAnHonestXingHeaderIsStillTrusted is the other side of the cross-check: it
// must not have made VBR measurement fall back to the walk in every case, which
// would quietly cost accuracy on exactly the files Xing exists for.
func TestAnHonestXingHeaderIsStillTrusted(t *testing.T) {
	const frames = 400
	data := mp3WithXing(frames, frames, uint32(frames*417))

	_, ms, err := probe.Audio(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("an honest Xing file was rejected: %v", err)
	}
	want := frames * 1152 * 1000 / 44100
	if ms < want*99/100 || ms > want*101/100 {
		t.Errorf("duration = %d ms, want ~%d ms", ms, want)
	}
}
