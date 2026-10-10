package imagesafe

import (
	"image"
	"image/color"
)

const bytesPerBlock = 64 * 4

func memoryNeeded(data []byte, kind string, cfg image.Config) (need int64, orientation int, err error) {
	if kind == kindPNG {
		return int64(cfg.Width) * int64(cfg.Height) * pngBytesPerPixel(cfg.ColorModel), 1, nil
	}
	frame, ok := scanJPEG(data)
	if !ok {
		return 0, 1, ErrUnreadable
	}
	return frame.decodeBytes(cfg.Width, cfg.Height), frame.orientation, nil
}

func pngBytesPerPixel(model color.Model) int64 {
	if _, paletted := model.(color.Palette); paletted {
		return 1
	}
	switch model {
	case color.GrayModel:
		return 1
	case color.Gray16Model:
		return 2
	case color.RGBAModel, color.NRGBAModel:
		return 4
	default:
		return 8
	}
}

type jpegComponent struct {
	id byte
	h  int
	v  int
}

type jpegFrame struct {
	progressive    bool
	jfif           bool
	adobe          bool
	adobeTransform byte
	exifSeen       bool
	comps          []jpegComponent
	orientation    int
}

const (
	markerSOF0  = 0xc0
	markerSOF1  = 0xc1
	markerSOF2  = 0xc2
	markerSOS   = 0xda
	markerEOI   = 0xd9
	markerAPP0  = 0xe0
	markerAPP1  = 0xe1
	markerAPP14 = 0xee
)

type jpegSegment struct {
	marker byte
	body   []byte
}

func markerAt(data []byte, pos int) (marker byte, next int, ok bool) {
	for pos < len(data) && data[pos] != 0xff {
		pos++
	}
	for pos < len(data) && data[pos] == 0xff {
		pos++
	}
	if pos >= len(data) {
		return 0, pos, false
	}
	return data[pos], pos + 1, true
}

func nextSegment(data []byte, pos int) (segment jpegSegment, next int, ok bool) {
	for {
		marker, after, found := markerAt(data, pos)
		if !found || marker == markerEOI || marker == markerSOS {
			return jpegSegment{}, after, false
		}
		pos = after
		if marker == 0 || (marker >= 0xd0 && marker <= 0xd7) {
			continue
		}
		if pos+2 > len(data) {
			return jpegSegment{}, pos, false
		}
		length := int(data[pos])<<8 | int(data[pos+1])
		if length < 2 || pos+length > len(data) {
			return jpegSegment{}, pos, false
		}
		return jpegSegment{marker: marker, body: data[pos+2 : pos+length]}, pos + length, true
	}
}

func scanJPEG(data []byte) (jpegFrame, bool) {
	frame := jpegFrame{orientation: 1}
	for pos := 2; ; {
		segment, next, ok := nextSegment(data, pos)
		if !ok {
			return frame, false
		}
		pos = next
		if segment.marker == markerSOF0 || segment.marker == markerSOF1 || segment.marker == markerSOF2 {
			frame.progressive = segment.marker == markerSOF2
			return frame, frame.readComponents(segment.body)
		}
		frame.absorb(segment)
	}
}

func (f *jpegFrame) absorb(segment jpegSegment) {
	body := segment.body
	switch segment.marker {
	case markerAPP0:
		f.jfif = f.jfif || (len(body) >= 5 && string(body[:5]) == "JFIF\x00")
	case markerAPP14:
		if len(body) >= 12 && string(body[:5]) == "Adobe" {
			f.adobe, f.adobeTransform = true, body[11]
		}
	case markerAPP1:
		if !f.exifSeen && len(body) >= 6 && string(body[:6]) == "Exif\x00\x00" {
			f.exifSeen = true
			f.orientation = exifOrientation(body[6:])
		}
	}
}

func (f *jpegFrame) readComponents(body []byte) bool {
	if len(body) < 6 {
		return false
	}
	count := int(body[5])
	if (count != 1 && count != 3 && count != 4) || len(body) != 6+3*count {
		return false
	}
	for i := range count {
		spec := body[6+3*i : 9+3*i]
		h, v := int(spec[1]>>4), int(spec[1]&0x0f)
		if h < 1 || h > 4 || v < 1 || v > 4 {
			return false
		}
		f.comps = append(f.comps, jpegComponent{id: spec[0], h: h, v: v})
	}
	return true
}

func (f jpegFrame) decodeBytes(width, height int) int64 {
	maxH, maxV := 1, 1
	if len(f.comps) > 1 {
		for _, c := range f.comps {
			maxH, maxV = max(maxH, c.h), max(maxV, c.v)
		}
	}
	mcuX, mcuY := ceilDiv(width, 8*maxH), ceilDiv(height, 8*maxV)
	plane := int64(8*maxH*mcuX) * int64(8*maxV*mcuY)
	pixels := int64(width) * int64(height)

	var total int64
	switch len(f.comps) {
	case 1:
		total = int64(8*mcuX) * int64(8*mcuY)
	default:
		total = f.yccBytes(plane, maxH, maxV)
		if len(f.comps) == 4 {
			total += plane*int64(f.comps[3].h*f.comps[3].v)/int64(maxH*maxV) + 4*pixels
		} else if f.rgb() {
			total += 4 * pixels
		}
	}
	if f.progressive {
		for _, c := range f.comps {
			total += int64(mcuX) * int64(mcuY) * int64(c.h*c.v) * bytesPerBlock
		}
	}
	return total
}

func (f jpegFrame) yccBytes(plane int64, maxH, maxV int) int64 {
	y, cb, cr := f.comps[0], f.comps[1], f.comps[2]
	if cb.h != cr.h || cb.v != cr.v || y.h != maxH || y.v != maxV {
		return 3 * plane
	}
	return plane + 2*plane*int64(cb.h*cb.v)/int64(maxH*maxV)
}

func (f jpegFrame) rgb() bool {
	if len(f.comps) != 3 || f.jfif {
		return false
	}
	if f.adobe && f.adobeTransform == 0 {
		return true
	}
	return f.comps[0].id == 'R' && f.comps[1].id == 'G' && f.comps[2].id == 'B'
}

func ceilDiv(a, b int) int {
	return (a + b - 1) / b
}
