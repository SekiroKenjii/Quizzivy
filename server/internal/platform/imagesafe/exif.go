package imagesafe

import "encoding/binary"

const (
	maxExifBytes   = 1 << 16
	maxIFDEntries  = 256
	tagOrientation = 0x0112
	ifdEntryBytes  = 12
	tiffMagic      = 42
	shortType      = 3
)

func exifOrientation(tiff []byte) int {
	if len(tiff) < 8 || len(tiff) > maxExifBytes {
		return 1
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	if order.Uint16(tiff[2:4]) != tiffMagic {
		return 1
	}
	ifd := int(order.Uint32(tiff[4:8]))
	if ifd < 8 || ifd > len(tiff)-2 {
		return 1
	}
	entries := int(order.Uint16(tiff[ifd : ifd+2]))
	pos := ifd + 2
	if entries > maxIFDEntries || pos+entries*ifdEntryBytes > len(tiff) {
		return 1
	}
	for range entries {
		entry := tiff[pos : pos+ifdEntryBytes]
		pos += ifdEntryBytes
		if order.Uint16(entry[0:2]) == tagOrientation {
			return orientationValue(order, entry)
		}
	}
	return 1
}

func orientationValue(order binary.ByteOrder, entry []byte) int {
	if order.Uint16(entry[2:4]) != shortType || order.Uint32(entry[4:8]) != 1 {
		return 1
	}
	value := int(order.Uint16(entry[8:10]))
	if value < 1 || value > 8 {
		return 1
	}
	return value
}
