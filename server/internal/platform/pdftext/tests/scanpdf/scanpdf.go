// Package scanpdf builds PDFs that hold only a picture of each page, with no text layer, as a scanner or a phone produces.
package scanpdf

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"strings"
)

// Size is the pixel size of the picture on each page, and Color says whether it has three channels.
type Size struct {
	Width, Height int
	Color         bool
}

// Small is a picture of 150 by 210 pixels in gray, enough to tell pages apart.
var Small = Size{Width: 150, Height: 210}

// Build returns a PDF of pages pages, each an A4 sheet holding one Small picture of black bars on white. The bars of page n start n rows lower than those of page 1, so no two pages look alike.
func Build(pages int) []byte { return BuildSized(pages, Small) }

// BuildSized is Build with a picture of the given size, such as an A4 sheet scanned at 300 dpi or a 12-megapixel photograph.
func BuildSized(pages int, size Size) []byte {
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"",
	}
	refs := make([]string, pages)
	for i := range pages {
		page, content, image := 3+3*i, 4+3*i, 5+3*i
		refs[i] = fmt.Sprintf("%d 0 R", page)
		draw := "q 595 0 0 842 0 0 cm /Im0 Do Q"
		objects = append(objects,
			fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /XObject << /Im0 %d 0 R >> >> /Contents %d 0 R >>", image, content),
			fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(draw), draw),
			picture(i+1, size))
	}
	objects[1] = fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(refs, " "), pages)
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, body := range objects {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return out.Bytes()
}

func picture(page int, size Size) string {
	channels, space := 1, "DeviceGray"
	if size.Color {
		channels, space = 3, "DeviceRGB"
	}
	width, height := size.Width, size.Height
	pixels := bytes.Repeat([]byte{0xff}, width*height*channels)
	pitch, gap, thickness := 20, height/10, max(height/35, 1)
	for bar := range 8 {
		top := pitch + page + bar*gap
		for row := top; row < top+thickness && row < height; row++ {
			for column := width / 8; column < width-width/8; column++ {
				for c := range channels {
					pixels[(row*width+column)*channels+c] = 0
				}
			}
		}
	}
	var packed bytes.Buffer
	deflate := zlib.NewWriter(&packed)
	_, _ = deflate.Write(pixels)
	_ = deflate.Close()
	return fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /%s /BitsPerComponent 8 /Filter /FlateDecode /Length %d >>\nstream\n%s\nendstream", width, height, space, packed.Len(), packed.String())
}
