package imagesafe_test

import (
	"bytes"
	"encoding/binary"
)

func segmentOf(marker byte, body []byte) []byte {
	out := []byte{0xff, marker}
	out = binary.BigEndian.AppendUint16(out, uint16(len(body)+2))
	return append(out, body...)
}

func eobRunJPEG(side, scans int) []byte {
	var out bytes.Buffer
	out.Write([]byte{0xff, 0xd8})
	out.Write(segmentOf(0xdb, append([]byte{0x00}, bytes.Repeat([]byte{1}, 64)...)))
	frame := []byte{8}
	frame = binary.BigEndian.AppendUint16(frame, uint16(side))
	frame = binary.BigEndian.AppendUint16(frame, uint16(side))
	out.Write(segmentOf(0xc2, append(frame, 1, 1, 0x11, 0)))
	counts := make([]byte, 16)
	counts[0] = 1
	out.Write(segmentOf(0xc4, append(append([]byte{0x10}, counts...), 0xe0)))
	scan := segmentOf(0xda, []byte{1, 1, 0x00, 1, 63, 0x00})
	blocks := ((side + 7) / 8) * ((side + 7) / 8)
	bits := (blocks + 16383) / 16384 * 15
	entropy := make([]byte, (bits+7)/8)
	if spare := len(entropy)*8 - bits; spare > 0 {
		entropy[len(entropy)-1] |= byte(1<<spare - 1)
	}
	for range scans {
		out.Write(scan)
		out.Write(entropy)
	}
	out.Write([]byte{0xff, 0xd9})
	return out.Bytes()
}

func manyScanBaseline(side, scans int) []byte {
	out := jpegFrame(frameSpec{width: side, height: side, sampling: [][2]int{{1, 1}}})
	for range scans {
		out = append(out, 0xff, 0xda, 0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3f, 0x00, 0x00)
	}
	return append(out, 0xff, 0xd9)
}
