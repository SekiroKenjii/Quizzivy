//go:build e2e

package e2e

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"math/rand/v2"
	"net/http"
	"quizzivy/internal/platform/config"
	"testing"
	"time"
)

func noisePNG(t *testing.T, seed uint64) []byte {
	t.Helper()
	random := rand.New(rand.NewPCG(seed, seed))
	img := image.NewRGBA(image.Rect(0, 0, 316, 316))
	for y := range 316 {
		for x := range 316 {
			img.Set(x, y, color.RGBA{R: uint8(random.UintN(256)), G: uint8(random.UintN(256)), B: uint8(random.UintN(256)), A: 255})
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func downloaded(t *testing.T, url string) []byte {
	t.Helper()
	response, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("downloading %s: %d %v", url, response.StatusCode, err)
	}
	return body
}

func near(t *testing.T, field any, want time.Time, within time.Duration) {
	t.Helper()
	text, ok := field.(string)
	if !ok {
		t.Fatalf("%v is not a time", field)
	}
	at, err := time.Parse(time.RFC3339Nano, text)
	if err != nil || at.Sub(want).Abs() > within {
		t.Fatalf("%s is not within %v of %v (%v)", text, within, want, err)
	}
}

func TestAReplacedFileAPublishedTestHoldsCountsTowardTheQuotaUntilNothingUsesIt(t *testing.T) {
	const quotaMiB = 1
	w := boot(t, func(c *config.Config) {
		c.S3Endpoint = storageEnv("S3_ENDPOINT", "http://localhost:9000")
		c.S3Region = storageEnv("S3_REGION", "us-east-1")
		c.S3Bucket = storageEnv("S3_BUCKET", "quizzivy-media")
		c.S3AccessKeyID = storageEnv("S3_ACCESS_KEY_ID", "quizzivy")
		c.S3SecretAccessKey = storageEnv("S3_SECRET_ACCESS_KEY", "quizzivy-dev-secret")
		c.S3ForcePathStyle = true
		c.SignedURLTTL = time.Minute
		c.MediaOwnerQuotaMiB = quotaMiB
	})
	email, password := w.createStaff("teacher")
	teacher := w.signedIn(email, password)
	held, fresh, spare, spareAgain, other := noisePNG(t, 1), noisePNG(t, 2), noisePNG(t, 3), noisePNG(t, 4), noisePNG(t, 5)
	size := int64(len(held))
	if quota := int64(quotaMiB << 20); 3*size > quota || 4*size <= quota {
		t.Fatalf("a file is %d bytes: three must fit the %d byte quota and four must not", size, quota)
	}
	upload := func(name string, data []byte) map[string]any {
		return replacementSend(t, teacher, http.StatusCreated, "/teacher/media", new(filePayload(t, name, data)))
	}
	usage := func() map[string]any {
		return teacher.must(http.StatusOK, http.MethodGet, "/teacher/media", nil)["usage"].(map[string]any)
	}
	library := func() map[string]bool {
		in := map[string]bool{}
		for _, row := range teacher.must(http.StatusOK, http.MethodGet, "/teacher/media", nil)["items"].([]any) {
			in[id(row.(map[string]any))] = true
		}
		return in
	}

	first := upload("giu-lai.png", held)
	second := upload("khac-1.png", spare)
	third := upload("khac-2.png", other)
	question := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/questions", map[string]any{"type": "single_choice", "prompt": "Ảnh nào?", "points": 1, "mediaAssetId": id(first),
		"options": []map[string]any{{"text": "Một", "isCorrect": true}, {"text": "Hai", "isCorrect": false}}})
	test := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/tests", map[string]any{"title": "Đề có ảnh " + nonce(t)})
	teacher.must(http.StatusOK, http.MethodPatch, "/teacher/tests/"+id(test), map[string]any{"expectedUpdatedAt": test["updatedAt"], "sections": []map[string]any{{"title": "Phần 1", "questionIds": []string{id(question)}}}})
	teacher.must(http.StatusCreated, http.MethodPost, "/teacher/tests/"+id(test)+"/publish", nil)

	refused := replacementSend(t, teacher, http.StatusConflict, "/teacher/media/"+id(first)+"/replace", new(filePayload(t, "moi.png", fresh)))
	if refused["error"].(map[string]any)["code"] != "MEDIA_QUOTA_EXCEEDED" {
		t.Fatalf("replacing a file a published test holds into a library with room for the new file alone: %v", refused)
	}
	if got := library(); !got[id(first)] || len(got) != 3 {
		t.Fatalf("the refused replacement changed the library: %v", got)
	}

	removed := teacher.must(http.StatusOK, http.MethodDelete, "/teacher/media/"+id(third), nil)
	if removed["id"] != id(third) || removed["bytes"] != float64(size) || len(removed["usedIn"].([]any)) != 0 || removed["url"] == "" {
		t.Fatalf("the delete answers %v, want the file, its size, a URL and no test", removed)
	}
	near(t, removed["deleteAt"], time.Now().Add(7*24*time.Hour), time.Minute)
	if !bytes.Equal(downloaded(t, removed["url"].(string)), other) {
		t.Error("the URL the delete answers does not download the deleted file")
	}
	figures := usage()
	if figures["imageBytes"] != float64(2*size) || figures["pendingBytes"] != float64(size) || figures["heldBytes"] != float64(0) {
		t.Fatalf("after the delete the usage is %v, want 2 files in the library, 1 pending and none held", figures)
	}
	near(t, figures["pendingDeleteAt"], time.Now().Add(7*24*time.Hour), time.Minute)

	replaced := replacementSend(t, teacher, http.StatusCreated, "/teacher/media/"+id(first)+"/replace", new(filePayload(t, "moi.png", fresh)))
	previous := replaced["previous"].(map[string]any)
	tests := previous["usedIn"].([]any)
	if previous["id"] != id(first) || previous["bytes"] != float64(size) || previous["deleteAt"] != nil || len(tests) != 1 || tests[0].(map[string]any)["id"] != id(test) || tests[0].(map[string]any)["version"] != float64(1) {
		t.Fatalf("the replacement answers the old file as %v, want it held by version 1 of the test with no removal time", previous)
	}
	if !bytes.Equal(downloaded(t, previous["url"].(string)), held) {
		t.Error("the URL the replacement answers does not download the old file")
	}
	figures = usage()
	if figures["imageBytes"] != float64(2*size) || figures["heldBytes"] != float64(size) || figures["pendingBytes"] != float64(size) {
		t.Fatalf("after the replacement the usage is %v, want 2 in the library, 1 held and 1 pending", figures)
	}
	if got := library(); got[id(first)] || !got[id(replaced["asset"].(map[string]any))] || !got[id(second)] || len(got) != 2 {
		t.Fatalf("the library is %v, want the new file and the other one", got)
	}

	filled := replacementSend(t, teacher, http.StatusConflict, "/teacher/media", new(filePayload(t, "day.png", spareAgain)))
	if filled["error"].(map[string]any)["code"] != "MEDIA_QUOTA_EXCEEDED" {
		t.Fatalf("an upload that would pass the quota once the held file counts: %v", filled)
	}
	upload("be.png", tinyPNG(t))

	loose := replacementSend(t, teacher, http.StatusCreated, "/teacher/media/"+id(second)+"/replace", new(filePayload(t, "khac-moi.png", spareAgain)))
	gone := loose["previous"].(map[string]any)
	if gone["id"] != id(second) || len(gone["usedIn"].([]any)) != 0 || gone["deleteAt"] == nil {
		t.Fatalf("the replacement of a file nothing uses answers the old file as %v, want no test and a removal time", gone)
	}
	near(t, gone["deleteAt"], time.Now().Add(7*24*time.Hour), time.Minute)
}
