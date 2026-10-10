package imagesafe

import (
	"image"
	"image/color"
)

type span struct {
	first   int
	weights []uint64
}

func overlap(a0, a1, b0, b1 int) int {
	return max(0, min(a1, b1)-max(a0, b0))
}

func spansOf(m, side int) []span {
	spans := make([]span, side)
	for i := range spans {
		lo, hi := i*m, (i+1)*m
		first, last := lo/side, (hi-1)/side
		weights := make([]uint64, last-first+1)
		for k := first; k <= last; k++ {
			weights[k-first] = uint64(overlap(lo, hi, k*side, (k+1)*side))
		}
		spans[i] = span{first: first, weights: weights}
	}
	return spans
}

func boxSquare(src image.Image, side int) *image.NRGBA {
	bounds := src.Bounds()
	m := min(bounds.Dx(), bounds.Dy())
	x0 := bounds.Min.X + (bounds.Dx()-m)/2
	y0 := bounds.Min.Y + (bounds.Dy()-m)/2
	columns := spansOf(m, side)
	read := rowReader(src)

	row := make([]uint32, 4*m)
	across := make([]uint64, 4*side)
	sums := make([]uint64, 4*side*side)
	for y := range m {
		read(row, x0, y0+y)
		clear(across)
		for i, c := range columns {
			for k, weight := range c.weights {
				px := row[4*(c.first+k) : 4*(c.first+k)+4]
				across[4*i] += weight * uint64(px[0])
				across[4*i+1] += weight * uint64(px[1])
				across[4*i+2] += weight * uint64(px[2])
				across[4*i+3] += weight * uint64(px[3])
			}
		}
		lo, hi := y*side, (y+1)*side
		for j := lo / m; j <= (hi-1)/m; j++ {
			weight := uint64(overlap(lo, hi, j*m, (j+1)*m))
			line := sums[4*side*j : 4*side*(j+1)]
			for n, v := range across {
				line[n] += weight * v
			}
		}
	}
	return straighten(sums, side, uint64(m)*uint64(m))
}

func straighten(sums []uint64, side int, total uint64) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, side, side))
	for i := range side * side {
		r, g, b, a := mean(sums[4*i], total), mean(sums[4*i+1], total), mean(sums[4*i+2], total), mean(sums[4*i+3], total)
		if a == 0 {
			continue
		}
		out.Pix[4*i] = to8(min(r*0xffff/a, 0xffff))
		out.Pix[4*i+1] = to8(min(g*0xffff/a, 0xffff))
		out.Pix[4*i+2] = to8(min(b*0xffff/a, 0xffff))
		out.Pix[4*i+3] = to8(a)
	}
	return out
}

func mean(sum, total uint64) uint64 {
	return (sum + total/2) / total
}

func to8(v uint64) uint8 {
	return uint8((v + 128) / 257)
}

func rowReader(src image.Image) func(dst []uint32, x0, y int) {
	switch img := src.(type) {
	case *image.NRGBA:
		return nrgbaRows(img)
	case *image.RGBA:
		return rgbaRows(img)
	case *image.YCbCr:
		return ycbcrRows(img)
	case *image.Gray:
		return grayRows(img)
	default:
		return anyRows(src)
	}
}

func nrgbaRows(img *image.NRGBA) func(dst []uint32, x0, y int) {
	return func(dst []uint32, x0, y int) {
		off := img.PixOffset(x0, y)
		for i := 0; i < len(dst); i += 4 {
			a := uint32(img.Pix[off+3]) * 0x101
			dst[i] = uint32(img.Pix[off]) * 0x101 * a / 0xffff
			dst[i+1] = uint32(img.Pix[off+1]) * 0x101 * a / 0xffff
			dst[i+2] = uint32(img.Pix[off+2]) * 0x101 * a / 0xffff
			dst[i+3] = a
			off += 4
		}
	}
}

func rgbaRows(img *image.RGBA) func(dst []uint32, x0, y int) {
	return func(dst []uint32, x0, y int) {
		off := img.PixOffset(x0, y)
		for i := 0; i < len(dst); i += 4 {
			dst[i], dst[i+1], dst[i+2], dst[i+3] = uint32(img.Pix[off])*0x101, uint32(img.Pix[off+1])*0x101, uint32(img.Pix[off+2])*0x101, uint32(img.Pix[off+3])*0x101
			off += 4
		}
	}
}

func ycbcrRows(img *image.YCbCr) func(dst []uint32, x0, y int) {
	return func(dst []uint32, x0, y int) {
		for i := 0; i < len(dst); i += 4 {
			x := x0 + i/4
			r, g, b := color.YCbCrToRGB(img.Y[img.YOffset(x, y)], img.Cb[img.COffset(x, y)], img.Cr[img.COffset(x, y)])
			dst[i], dst[i+1], dst[i+2], dst[i+3] = uint32(r)*0x101, uint32(g)*0x101, uint32(b)*0x101, 0xffff
		}
	}
}

func grayRows(img *image.Gray) func(dst []uint32, x0, y int) {
	return func(dst []uint32, x0, y int) {
		off := img.PixOffset(x0, y)
		for i := 0; i < len(dst); i += 4 {
			v := uint32(img.Pix[off]) * 0x101
			dst[i], dst[i+1], dst[i+2], dst[i+3] = v, v, v, 0xffff
			off++
		}
	}
}

func anyRows(src image.Image) func(dst []uint32, x0, y int) {
	return func(dst []uint32, x0, y int) {
		for i := 0; i < len(dst); i += 4 {
			dst[i], dst[i+1], dst[i+2], dst[i+3] = src.At(x0+i/4, y).RGBA()
		}
	}
}

func orient(img *image.NRGBA, orientation int) *image.NRGBA {
	if orientation < 2 || orientation > 8 {
		return img
	}
	n := img.Bounds().Dx()
	out := image.NewNRGBA(img.Bounds())
	for y := range n {
		for x := range n {
			sx, sy := sourceOf(orientation, x, y, n)
			copy(out.Pix[out.PixOffset(x, y):out.PixOffset(x, y)+4], img.Pix[img.PixOffset(sx, sy):img.PixOffset(sx, sy)+4])
		}
	}
	return out
}

func sourceOf(orientation, x, y, n int) (int, int) {
	switch orientation {
	case 2:
		return n - 1 - x, y
	case 3:
		return n - 1 - x, n - 1 - y
	case 4:
		return x, n - 1 - y
	case 5:
		return y, x
	case 6:
		return y, n - 1 - x
	case 7:
		return n - 1 - y, n - 1 - x
	default:
		return n - 1 - y, x
	}
}
