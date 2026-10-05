package probe

import (
	"encoding/binary"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
)

// Image reads an image's size in pixels from its header: PNG and JPEG
// through the standard decoders' configuration, WebP from its `VP8 `, `VP8L`
// or `VP8X` chunk. It never decodes pixels. ok is false for any other
// content and for a header that is cut short or names no positive size.
func Image(r io.ReaderAt, size int64) (width, height int, ok bool) {
	if size <= 0 {
		return 0, 0, false
	}
	head := make([]byte, webpHeader)
	n, _ := r.ReadAt(head, 0)
	head = head[:n]
	if len(head) >= 12 && string(head[0:4]) == "RIFF" && string(head[8:12]) == "WEBP" {
		return webpSize(head)
	}
	cfg, format, err := image.DecodeConfig(io.NewSectionReader(r, 0, size))
	if err != nil || (format != "png" && format != "jpeg") || cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0, false
	}
	return cfg.Width, cfg.Height, true
}

const webpHeader = 30

func webpSize(head []byte) (width, height int, ok bool) {
	if len(head) < webpHeader {
		return 0, 0, false
	}
	payload := head[20:]
	switch string(head[12:16]) {
	case "VP8 ":
		if payload[0]&1 != 0 || payload[3] != 0x9d || payload[4] != 0x01 || payload[5] != 0x2a {
			return 0, 0, false
		}
		width = int(binary.LittleEndian.Uint16(payload[6:8]) & 0x3fff)
		height = int(binary.LittleEndian.Uint16(payload[8:10]) & 0x3fff)
	case "VP8L":
		if payload[0] != 0x2f {
			return 0, 0, false
		}
		bits := binary.LittleEndian.Uint32(payload[1:5])
		width = int(bits&0x3fff) + 1
		height = int(bits>>14&0x3fff) + 1
	case "VP8X":
		width = int(uint32(payload[4])|uint32(payload[5])<<8|uint32(payload[6])<<16) + 1
		height = int(uint32(payload[7])|uint32(payload[8])<<8|uint32(payload[9])<<16) + 1
	default:
		return 0, 0, false
	}
	if width <= 0 || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}
