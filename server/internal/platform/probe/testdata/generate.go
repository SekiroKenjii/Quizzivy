//go:build ignore

package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

// An MPEG-1 Layer III frame header.
//
//	byte 0: 11111111                     sync
//	byte 1: 111 11 01 1                  sync, MPEG1, Layer III, no CRC
//	byte 2: bitrateIdx sampleRateIdx pad private
//	byte 3: channelMode ...
func frameHeader(bitrateIdx, sampleRateIdx, padding, channelMode byte) []byte {
	return []byte{
		0xFF,
		0xFB,
		bitrateIdx<<4 | sampleRateIdx<<2 | padding<<1,
		channelMode << 6,
	}
}

var mpeg1LayerIIIBitrates = []int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320}

func frameLen(bitrateIdx byte, sampleRate, padding int) int {
	return 144*mpeg1LayerIIIBitrates[bitrateIdx]*1000/sampleRate + padding
}

// cbr builds `frames` identical frames at one bitrate. Stereo, 44.1 kHz.
func cbr(bitrateIdx byte, frames int) []byte {
	const sampleRate = 44100
	size := frameLen(bitrateIdx, sampleRate, 0)
	out := make([]byte, 0, size*frames)
	for range frames {
		f := make([]byte, size)
		copy(f, frameHeader(bitrateIdx, 0, 0, 0))
		out = append(out, f...)
	}
	return out
}

// withXing rebuilds the first frame to carry a Xing header declaring
// `declared` frames -- the count a player trusts for a VBR file.
func withXing(stream []byte, bitrateIdx byte, declared int) []byte {
	const sideInfoStereoMPEG1 = 32
	out := make([]byte, len(stream))
	copy(out, stream)

	at := 4 + sideInfoStereoMPEG1
	copy(out[at:], []byte("Xing"))
	binary.BigEndian.PutUint32(out[at+4:], 0x0001) // frames-present flag
	binary.BigEndian.PutUint32(out[at+8:], uint32(declared))
	return out
}

// vbr builds frames whose bitrate alternates, with no Xing header -- the case
// that has to be measured by walking every frame.
func vbr(frames int) []byte {
	const sampleRate = 44100
	indices := []byte{5, 9, 12, 7} // 64, 128, 224, 96 kbps
	out := []byte{}
	for i := range frames {
		idx := indices[i%len(indices)]
		f := make([]byte, frameLen(idx, sampleRate, 0))
		copy(f, frameHeader(idx, 0, 0, 0))
		out = append(out, f...)
	}
	return out
}

// id3v2 wraps a stream in a tag of `payload` bytes. The size is syncsafe:
// seven bits per byte, high bit clear.
func id3v2(payload int, stream []byte) []byte {
	header := make([]byte, 10)
	copy(header, []byte("ID3"))
	header[3], header[4] = 0x03, 0x00 // v2.3.0
	header[5] = 0x00                  // no flags, no footer
	header[6] = byte((payload >> 21) & 0x7F)
	header[7] = byte((payload >> 14) & 0x7F)
	header[8] = byte((payload >> 7) & 0x7F)
	header[9] = byte(payload & 0x7F)

	body := make([]byte, payload)
	// Bytes that would look like a frame sync if the tag were not skipped.
	for i := range body {
		body[i] = 0xFF
	}
	return append(append(header, body...), stream...)
}

func atom(name string, body []byte) []byte {
	out := make([]byte, 8, 8+len(body))
	binary.BigEndian.PutUint32(out[0:4], uint32(8+len(body)))
	copy(out[4:8], name)
	return append(out, body...)
}

// mp4 builds ftyp + moov/mvhd. `wide` selects the 64-bit mvhd (version 1).
func mp4(timescale, duration uint64, wide bool) []byte {
	ftyp := atom("ftyp", append([]byte("M4A "), append(
		[]byte{0, 0, 0, 0}, []byte("M4A mp42isom")...)...))

	var mvhd []byte
	if wide {
		body := make([]byte, 4+8+8+4+8+80)
		body[0] = 1 // version
		binary.BigEndian.PutUint32(body[20:24], uint32(timescale))
		binary.BigEndian.PutUint64(body[24:32], duration)
		mvhd = atom("mvhd", body)
	} else {
		body := make([]byte, 100)
		body[0] = 0 // version
		binary.BigEndian.PutUint32(body[12:16], uint32(timescale))
		binary.BigEndian.PutUint32(body[16:20], uint32(duration))
		mvhd = atom("mvhd", body)
	}
	mdat := atom("mdat", make([]byte, 2048))
	return append(append(ftyp, mdat...), atom("moov", mvhd)...)
}

// wav is a RIFF file, for the "renamed to .mp3" case.
func wav() []byte {
	out := []byte("RIFF")
	out = binary.BigEndian.AppendUint32(out, 2048)
	out = append(out, []byte("WAVEfmt ")...)
	return append(out, make([]byte, 2048)...)
}

func write(name string, data []byte) {
	path := filepath.Join("testdata", name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("%-28s %6d bytes\n", name, len(data))
}

func main() {
	// 383 frames x 1152 samples / 44100 Hz = 10.006 s
	write("cbr-128k.mp3", cbr(9, 383))
	write("vbr-xing.mp3", withXing(cbr(9, 383), 9, 383))

	// No Xing: only walking the frames gets this right.
	write("vbr-no-xing.mp3", vbr(383))

	// A 32 KB tag full of 0xFF, which is a false frame sync on every byte.
	write("id3v2-large.mp3", id3v2(32*1024, cbr(9, 383)))

	// 10.000 s at two common timescales.
	write("mvhd-v0.m4a", mp4(1000, 10_000, false))
	write("mvhd-v1.m4a", mp4(44100, 441_000, true))

	write("wav-renamed.mp3", wav())

	// A valid first frame, then nothing -- the file stops mid-stream.
	full := cbr(9, 383)
	write("truncated.mp3", full[:600])
}
