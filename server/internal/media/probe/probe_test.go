package probe_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"quizzivy/internal/media/probe"
)

// §11.1: identify by magic bytes, never by extension or Content-Type, and
// measure without shelling out to ffprobe.
//
// **What this corpus is.** The fixtures are SYNTHESISED, not encoded --
// testdata/generate.go builds them byte by byte, so every duration here is
// known by construction rather than by trusting an encoder. That makes each
// case exact and isolated.
//
// **What it therefore does not cover.** Real encoder quirks: LAME's exact
// padding, iTunes' atom ordering, the odd stray byte between frames that
// appears in files from the wild. R-08 is about VBR-without-Xing being wrong,
// and this corpus proves the ARITHMETIC is right without proving the parser
// survives everything a real encoder emits. Dropping a handful of real files
// into testdata/ and adding them to the table below is the way to close that,
// and needs no code change.

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
		// The whole reason identification is by magic bytes. The extension says
		// mp3 and the file is RIFF; believing the extension would store a file
		// no browser will play as audio.
		r, size := open(t, "wav-renamed.mp3")
		_, _, err := probe.Audio(r, size)
		if !errors.Is(err, probe.ErrUnsupportedType) {
			t.Fatalf("error = %v, want ErrUnsupportedType", err)
		}
	})

	t.Run("a truncated MP3", func(t *testing.T) {
		// It sniffs correctly and its first frames parse, so this is exactly
		// the "sniffs but cannot be probed" case: REJECT rather than store with
		// a null duration. media_assets CHECKs that an audio row has one, so
		// storing it would fail at the database and the teacher would get a
		// 500 instead of an explanation.
		r, size := open(t, "truncated.mp3")
		_, _, err := probe.Audio(r, size)
		if !errors.Is(err, probe.ErrUnmeasurable) {
			t.Fatalf("error = %v, want ErrUnmeasurable", err)
		}
	})
}

func TestNothingOutsideTheAllowlistIsAccepted(t *testing.T) {
	// §11.1 is an allowlist, and §17.3 flags widening it as a deliberate
	// decision. These are the shapes most likely to be handed to an upload.
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
	// Stated as its own case because it is the §11.1 rule, and because probe
	// takes no filename at all -- which is the design that makes it true.
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
	// Bounded work (§14). Two shapes that would otherwise spin: an atom that
	// declares a size of zero, and a stream of valid-looking syncs.
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
		// Every frame is legal; there are simply far more than five minutes
		// of them. The prober must give up rather than walk them all.
		frame := []byte{0xFF, 0xFB, 0x90, 0x00}
		data := make([]byte, 0, 4*1024*1024)
		for len(data) < 4*1024*1024 {
			f := make([]byte, 417)
			copy(f, frame)
			data = append(data, f...)
		}
		_, ms, err := probe.Audio(bytes.NewReader(data), int64(len(data)))
		// Either answer is acceptable; hanging is not. What is asserted is
		// that it returns at all, and that any duration it reports is sane.
		if err == nil && ms <= 0 {
			t.Errorf("returned a nonsensical duration %d", ms)
		}
	})
}
