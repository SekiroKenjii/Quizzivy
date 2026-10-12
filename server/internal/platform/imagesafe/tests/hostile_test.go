package imagesafe_test

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"testing"
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

var adam7 = [7][4]int{{0, 0, 8, 8}, {4, 0, 8, 8}, {0, 4, 4, 8}, {2, 0, 4, 4}, {0, 2, 2, 4}, {1, 0, 2, 2}, {0, 1, 1, 2}}

func bitsPerPixel(colour, depth byte) int {
	switch colour {
	case 2:
		return 3 * int(depth)
	case 4:
		return 2 * int(depth)
	case 6:
		return 4 * int(depth)
	default:
		return int(depth)
	}
}

func rawRows(width, height int, bits int, interlaced bool) int {
	rowBytes := func(w int) int { return 1 + (w*bits+7)/8 }
	if !interlaced {
		return height * rowBytes(width)
	}
	total := 0
	for _, pass := range adam7 {
		w, h := (width-pass[0]+pass[2]-1)/pass[2], (height-pass[1]+pass[3]-1)/pass[3]
		if w > 0 && h > 0 {
			total += h * rowBytes(w)
		}
	}
	return total
}

func handPNG(t *testing.T, width, height int, colour, depth byte, interlaced bool, ahead ...[]byte) []byte {
	t.Helper()
	header := binary.BigEndian.AppendUint32(nil, uint32(width))
	header = binary.BigEndian.AppendUint32(header, uint32(height))
	header = append(header, depth, colour, 0, 0, 0)
	if interlaced {
		header[12] = 1
	}
	var packed bytes.Buffer
	writer := zlib.NewWriter(&packed)
	if _, err := writer.Write(make([]byte, rawRows(width, height, bitsPerPixel(colour, depth), interlaced))); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	out := append([]byte("\x89PNG\r\n\x1a\n"), pngChunk("IHDR", header)...)
	for _, chunk := range ahead {
		out = append(out, chunk...)
	}
	out = append(out, pngChunk("IDAT", packed.Bytes())...)
	return append(out, pngChunk("IEND", nil)...)
}

func headerOnlyPNG(width, height int, colour, depth byte, interlaced bool, ahead ...[]byte) []byte {
	header := binary.BigEndian.AppendUint32(nil, uint32(width))
	header = binary.BigEndian.AppendUint32(header, uint32(height))
	header = append(header, depth, colour, 0, 0, 0)
	if interlaced {
		header[12] = 1
	}
	out := append([]byte("\x89PNG\r\n\x1a\n"), pngChunk("IHDR", header)...)
	for _, chunk := range ahead {
		out = append(out, chunk...)
	}
	return out
}
