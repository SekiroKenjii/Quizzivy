//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strings"
	"testing"
)

func sidewaysPortrait(t *testing.T) []byte {
	t.Helper()
	raw := image.NewNRGBA(image.Rect(0, 0, 400, 300))
	for y := range 300 {
		for x := range 400 {
			if x < 200 {
				raw.SetNRGBA(x, y, color.NRGBA{B: 255, A: 255})
			} else {
				raw.SetNRGBA(x, y, color.NRGBA{G: 255, A: 255})
			}
		}
	}
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, raw, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	tiff := []byte{'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 0x01, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0}
	segment := append([]byte("Exif\x00\x00"), tiff...)
	segment = append(segment, []byte("SECRET-GPS-10.762622,106.660172")...)
	app1 := append([]byte{0xff, 0xe1, byte((len(segment) + 2) >> 8), byte(len(segment) + 2)}, segment...)
	photo := append(append(append([]byte(nil), encoded.Bytes()[:2]...), app1...), encoded.Bytes()[2:]...)
	return append(photo, []byte("SECRET-TRAILING-BYTES")...)
}

func declaredBomb() []byte {
	header := binary.BigEndian.AppendUint32(nil, 30000)
	header = binary.BigEndian.AppendUint32(header, 30000)
	header = append(header, 8, 6, 0, 0, 0)
	chunk := binary.BigEndian.AppendUint32(nil, uint32(len(header)))
	chunk = append(chunk, "IHDR"...)
	chunk = append(chunk, header...)
	chunk = binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(chunk[4:]))
	return append([]byte("\x89PNG\r\n\x1a\n"), chunk...)
}

func objectAt(t *testing.T, url string) (int, http.Header, []byte) {
	t.Helper()
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("fetching %s: %v", url, err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, response.Header, body
}

func chunkTypes(t *testing.T, data []byte) []string {
	t.Helper()
	var kinds []string
	for pos := 8; pos+8 <= len(data); {
		kinds = append(kinds, string(data[pos+4:pos+8]))
		pos += 12 + int(binary.BigEndian.Uint32(data[pos:pos+4]))
	}
	return kinds
}

func errorCodeOf(s sent) any {
	if e, ok := s.json["error"].(map[string]any); ok {
		return e["code"]
	}
	return nil
}

func TestAProfilePhotoIsStoredAsASquareCleanPngAndEveryChangeDeletesTheObjectItReplaced(t *testing.T) {
	w := bootWithStorage(t)
	email, password := w.createStaff("teacher")
	teacher := w.signedIn(email, password)
	otherEmail, otherPassword := w.createStaff("teacher")
	other := w.signedIn(otherEmail, otherPassword)

	first := teacher.send(http.MethodPut, "/me/avatar", new(filePayload(t, "anh.jpg", sidewaysPortrait(t))))
	if first.status != http.StatusOK {
		t.Fatalf("the first photo: %s", answer(first))
	}
	firstURL, _ := first.json["avatarUrl"].(string)
	if firstURL == "" || first.json["email"] != email || first.json["permissions"] == nil {
		t.Fatalf("the answer is not the whole caller with a signed url: %s", answer(first))
	}
	if _, present := first.json["avatarKey"]; present || strings.Contains(string(first.body), "avatar_key") {
		t.Fatalf("the answer carries the stored key as a field: %s", answer(first))
	}

	status, _, stored := objectAt(t, firstURL)
	if status != http.StatusOK {
		t.Fatalf("the stored photo answered %d", status)
	}
	if bytes.Contains(stored, []byte("SECRET")) || bytes.Contains(stored, []byte("Exif")) {
		t.Fatal("the stored photo still carries the upload's metadata")
	}
	for _, kind := range chunkTypes(t, stored) {
		if kind != "IHDR" && kind != "IDAT" && kind != "IEND" {
			t.Fatalf("the stored photo carries a %s chunk", kind)
		}
	}
	decoded, err := png.Decode(bytes.NewReader(stored))
	if err != nil || decoded.Bounds().Dx() != 256 || decoded.Bounds().Dy() != 256 {
		t.Fatalf("the stored photo is %v, %v; want a 256 x 256 png", decoded.Bounds(), err)
	}
	top, bottom := color.NRGBAModel.Convert(decoded.At(128, 30)).(color.NRGBA), color.NRGBAModel.Convert(decoded.At(128, 226)).(color.NRGBA)
	if top.B < 200 || top.G > 60 || bottom.G < 200 || bottom.B > 60 {
		t.Errorf("the portrait ended with %v over %v, want blue over green: the phone's orientation was not applied", top, bottom)
	}

	me := teacher.must(http.StatusOK, http.MethodGet, "/auth/me", nil)
	if me["avatarUrl"] == nil {
		t.Fatalf("GET /auth/me carries no avatarUrl: %v", me)
	}
	if otherMe := other.must(http.StatusOK, http.MethodGet, "/auth/me", nil); otherMe["avatarUrl"] != nil {
		t.Fatalf("another user has a photo: %v", otherMe)
	}

	second := teacher.send(http.MethodPut, "/me/avatar", new(filePayload(t, "mat.png", avatarPNG(t))))
	if second.status != http.StatusOK {
		t.Fatalf("the replacement: %s", answer(second))
	}
	secondURL, _ := second.json["avatarUrl"].(string)
	if secondURL == "" || strings.Split(secondURL, "?")[0] == strings.Split(firstURL, "?")[0] {
		t.Fatalf("the replacement kept the old object: %s", secondURL)
	}
	if status, _, _ := objectAt(t, firstURL); status != http.StatusNotFound {
		t.Errorf("the replaced photo answered %d, want it deleted", status)
	}
	if status, _, _ := objectAt(t, secondURL); status != http.StatusOK {
		t.Errorf("the new photo answered %d", status)
	}

	var animated bytes.Buffer
	if err := gif.Encode(&animated, image.NewPaletted(image.Rect(0, 0, 300, 300), color.Palette{color.Black}), nil); err != nil {
		t.Fatal(err)
	}
	exact := padded(avatarPNG(t), 2<<20)
	refusals := []struct {
		name   string
		data   []byte
		status int
		code   string
	}{
		{"a gif", animated.Bytes(), http.StatusUnsupportedMediaType, "MEDIA_TYPE_UNSUPPORTED"},
		{"a png declaring 30000 x 30000", declaredBomb(), http.StatusUnsupportedMediaType, "IMAGE_DIMENSIONS"},
		{"a png of 100 x 100", squarePNG(t, 100), http.StatusUnsupportedMediaType, "IMAGE_DIMENSIONS"},
		{"a progressive jpeg of 33 empty scans", emptyScanJPEG(2048, 33), http.StatusUnsupportedMediaType, "IMAGE_DIMENSIONS"},
		{"a file of 2 MiB and a byte", padded(avatarPNG(t), 2<<20+1), http.StatusRequestEntityTooLarge, "MEDIA_TOO_LARGE"},
		{"a request far beyond the allowance", bytes.Repeat([]byte{0x89}, 3<<20), http.StatusRequestEntityTooLarge, "MEDIA_TOO_LARGE"},
	}
	for _, r := range refusals {
		got := teacher.send(http.MethodPut, "/me/avatar", new(filePayload(t, "x", r.data)))
		if got.status != r.status || errorCodeOf(got) != r.code {
			t.Errorf("%s answered %s, want %d %s", r.name, answer(got), r.status, r.code)
		}
	}
	if kept := teacher.must(http.StatusOK, http.MethodGet, "/auth/me", nil); kept["avatarUrl"] == nil {
		t.Error("a refused upload removed the photo")
	}

	spent, photos := 2+len(refusals), 2
	if full := teacher.send(http.MethodPut, "/me/avatar", new(filePayload(t, "ba-ngan.png", exact))); full.status != http.StatusOK {
		t.Fatalf("a file of exactly 2 MiB: %s, want it accepted", answer(full))
	}
	spent++
	photos++
	for ; spent < 10; spent++ {
		if again := teacher.send(http.MethodPut, "/me/avatar", new(filePayload(t, "mat.png", avatarPNG(t)))); again.status != http.StatusOK {
			t.Fatalf("attempt %d: %s", spent+1, answer(again))
		}
		photos++
	}
	limited := teacher.send(http.MethodPut, "/me/avatar", new(filePayload(t, "mat.png", avatarPNG(t))))
	if limited.status != http.StatusTooManyRequests || errorCodeOf(limited) != "RATE_LIMITED" {
		t.Fatalf("the eleventh upload in the hour: %s, want 429 RATE_LIMITED", answer(limited))
	}
	if got := other.send(http.MethodPut, "/me/avatar", new(filePayload(t, "mat.png", avatarPNG(t)))); got.status != http.StatusOK {
		t.Fatalf("another user's first upload: %s, want their own budget", answer(got))
	}

	current := teacher.must(http.StatusOK, http.MethodGet, "/auth/me", nil)
	currentURL, _ := current["avatarUrl"].(string)
	removed := teacher.send(http.MethodDelete, "/me/avatar", nil)
	if removed.status != http.StatusOK || removed.json["avatarUrl"] != nil || removed.json["email"] != email {
		t.Fatalf("removing the photo: %s", answer(removed))
	}
	if status, _, _ := objectAt(t, currentURL); status != http.StatusNotFound {
		t.Errorf("the removed photo answered %d, want it deleted", status)
	}
	if again := teacher.send(http.MethodDelete, "/me/avatar", nil); again.status != http.StatusOK || again.json["avatarUrl"] != nil {
		t.Errorf("removing a photo that is not there: %s", answer(again))
	}

	var sets, removals int
	if err := w.pool.QueryRow(context.Background(), `SELECT count(*) FILTER (WHERE action = 'user.avatar_set'), count(*) FILTER (WHERE action = 'user.avatar_removed')
	  FROM app.audit_log WHERE entity = 'user' AND entity_id = $1::uuid`, current["id"]).Scan(&sets, &removals); err != nil {
		t.Fatal(err)
	}
	if sets != photos || removals != 1 {
		t.Errorf("the audit holds %d photo sets and %d removals, want %d and 1", sets, removals, photos)
	}
}

func squarePNG(t *testing.T, side int) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := png.Encode(&out, image.NewNRGBA(image.Rect(0, 0, side, side))); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func padded(data []byte, size int) []byte {
	return append(append([]byte(nil), data...), make([]byte, size-len(data))...)
}

func emptyScanJPEG(side, scans int) []byte {
	segment := func(marker byte, body []byte) []byte {
		out := binary.BigEndian.AppendUint16([]byte{0xff, marker}, uint16(len(body)+2))
		return append(out, body...)
	}
	var out bytes.Buffer
	out.Write([]byte{0xff, 0xd8})
	out.Write(segment(0xdb, append([]byte{0x00}, bytes.Repeat([]byte{1}, 64)...)))
	frame := binary.BigEndian.AppendUint16(binary.BigEndian.AppendUint16([]byte{8}, uint16(side)), uint16(side))
	out.Write(segment(0xc2, append(frame, 1, 1, 0x11, 0)))
	counts := make([]byte, 16)
	counts[0] = 1
	out.Write(segment(0xc4, append(append([]byte{0x10}, counts...), 0xe0)))
	blocks := ((side + 7) / 8) * ((side + 7) / 8)
	bits := (blocks + 16383) / 16384 * 15
	entropy := make([]byte, (bits+7)/8)
	if spare := len(entropy)*8 - bits; spare > 0 {
		entropy[len(entropy)-1] |= byte(1<<spare - 1)
	}
	for range scans {
		out.Write(segment(0xda, []byte{1, 1, 0x00, 1, 63, 0x00}))
		out.Write(entropy)
	}
	out.Write([]byte{0xff, 0xd9})
	return out.Bytes()
}
