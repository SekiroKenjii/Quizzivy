//go:build e2e

package e2e

import (
	"net/http"
	"os"
	"testing"
	"time"

	"quizzivy/internal/platform/config"
)

func TestTheMediaLibraryNamesMeasuresAndBoundsWhatATeacherUploads(t *testing.T) {
	const quotaMiB = 1
	w := boot(t, func(c *config.Config) {
		c.S3Endpoint = storageEnv("S3_ENDPOINT", "http://localhost:9000")
		c.S3Region = storageEnv("S3_REGION", "us-east-1")
		c.S3Bucket = storageEnv("S3_BUCKET", "quizzivy-media")
		c.S3AccessKeyID = storageEnv("S3_ACCESS_KEY_ID", "quizzivy")
		c.S3SecretAccessKey = storageEnv("S3_SECRET_ACCESS_KEY", "quizzivy-dev-secret")
		c.S3ForcePathStyle = true
		c.SignedURLTTL = 10 * time.Minute
		c.MediaOwnerQuotaMiB = quotaMiB
	})
	email, password := w.createStaff("teacher")
	teacher := w.signedIn(email, password)
	listening, err := os.ReadFile("../../web/tests/e2e/fixtures/unit5-listening.mp3")
	if err != nil {
		t.Fatal(err)
	}
	drawing := tinyPNG(t)
	upload := func(query, name string, data []byte) sent {
		return teacher.send(http.MethodPost, "/teacher/media"+query, new(filePayload(t, name, data)))
	}
	library := func(query string) (map[string]any, map[string]map[string]any) {
		body := teacher.must(http.StatusOK, http.MethodGet, "/teacher/media"+query, nil)
		rows := map[string]map[string]any{}
		for _, item := range body["items"].([]any) {
			row := item.(map[string]any)
			rows[row["id"].(string)] = row
		}
		return body, rows
	}
	figures := func(body map[string]any, name string) map[string]any {
		return body[name].(map[string]any)
	}

	picture := upload("", "ban-do.png", drawing)
	if picture.status != http.StatusCreated {
		t.Fatalf("the image upload: %s", answer(picture))
	}
	clip := upload("?defaultMaxPlays=2", "bai-nghe.mp3", listening)
	if clip.status != http.StatusCreated {
		t.Fatalf("the audio upload: %s", answer(clip))
	}
	pictureID, clipID := picture.json["id"].(string), clip.json["id"].(string)
	limited := upload("?defaultMaxPlays=2", "ban-do-2.png", drawing)
	if limited.status != http.StatusBadRequest || limited.json["error"].(map[string]any)["code"] != "VALIDATION_FAILED" {
		t.Fatalf("an image uploaded with a play limit: %s, want 400 VALIDATION_FAILED", answer(limited))
	}

	body, rows := library("")
	if body["total"] != float64(2) || len(rows) != 2 {
		t.Fatalf("the library holds %v files in %d rows, want the two that were accepted", body["total"], len(rows))
	}
	if row := rows[pictureID]; row["width"] != float64(2) || row["height"] != float64(2) || row["displayName"] != "ban-do.png" || row["defaultMaxPlays"] != nil || row["questionCount"] != float64(0) {
		t.Errorf("the image lists as %v, want 2 by 2 pixels under its filename, with no play limit and no question", row)
	}
	if row := rows[clipID]; row["defaultMaxPlays"] != float64(2) || row["width"] != nil || row["height"] != nil || row["displayName"] != "bai-nghe.mp3" {
		t.Errorf("the audio lists as %v, want a play limit of 2 and no size in pixels", row)
	}
	if facets := figures(body, "facets"); facets["all"] != float64(2) || facets["audio"] != float64(1) || facets["image"] != float64(1) || facets["unused"] != float64(2) {
		t.Errorf("the tabs count %v, want 2 files, 1 audio, 1 image and 2 unused", facets)
	}
	if usage := figures(body, "usage"); usage["audioBytes"] != float64(len(listening)) || usage["imageBytes"] != float64(len(drawing)) || usage["quotaBytes"] != float64(quotaMiB<<20) {
		t.Errorf("the library holds %v, want %d of audio and %d of images against the configured %d", usage, len(listening), len(drawing), quotaMiB<<20)
	}

	renamed := teacher.must(http.StatusOK, http.MethodPatch, "/teacher/media/"+pictureID, map[string]any{"displayName": "  Bản đồ chỉ đường  "})
	if renamed["displayName"] != "Bản đồ chỉ đường" || renamed["originalFilename"] != "ban-do.png" || renamed["url"] == "" {
		t.Errorf("the rename answers %v, want the trimmed name over the stored file", renamed)
	}
	teacher.must(http.StatusBadRequest, http.MethodPatch, "/teacher/media/"+pictureID, map[string]any{"defaultMaxPlays": 1})
	if cleared := teacher.must(http.StatusOK, http.MethodPatch, "/teacher/media/"+clipID, map[string]any{"defaultMaxPlays": nil}); cleared["defaultMaxPlays"] != nil {
		t.Errorf("a cleared play limit answers %v", cleared["defaultMaxPlays"])
	}
	teacher.must(http.StatusCreated, http.MethodPost, "/teacher/questions", map[string]any{
		"type": "single_choice", "prompt": "Bản đồ chỉ đường nào?", "points": 1, "mediaAssetId": pictureID,
		"options": []map[string]any{{"text": "bên trái", "isCorrect": false}, {"text": "bên phải", "isCorrect": true}},
	})

	body, rows = library("?q=ban+do+chi")
	if body["total"] != float64(1) || rows[pictureID]["questionCount"] != float64(1) {
		t.Errorf("a search without accents finds %v with rows %v, want the renamed image and its one question", body["total"], rows)
	}
	if facets := figures(body, "facets"); facets["all"] != float64(1) || facets["image"] != float64(1) || facets["audio"] != float64(0) || facets["unused"] != float64(0) {
		t.Errorf("under the search the tabs count %v, want the one image, in use", facets)
	}
	if usage := figures(body, "usage"); usage["audioBytes"] != float64(len(listening)) {
		t.Errorf("under the search the library holds %v, want the whole library's %d of audio", usage, len(listening))
	}
	body, rows = library("?unused=true")
	if body["total"] != float64(1) || rows[clipID] == nil {
		t.Errorf("unused lists %v, want only the audio no question uses", rows)
	}

	fits := (quotaMiB<<20 - len(drawing)) / len(listening)
	for n := 2; n <= fits; n++ {
		if again := upload("", "bai-nghe.mp3", listening); again.status != http.StatusCreated {
			t.Fatalf("audio upload %d of the %d that fit: %s", n, fits, answer(again))
		}
	}
	full := upload("", "bai-nghe.mp3", listening)
	if full.status != http.StatusConflict || full.json["error"].(map[string]any)["code"] != "MEDIA_QUOTA_EXCEEDED" {
		t.Fatalf("the upload past the quota: %s, want 409 MEDIA_QUOTA_EXCEEDED", answer(full))
	}
	body, _ = library("")
	if usage := figures(body, "usage"); body["total"] != float64(fits+1) || usage["audioBytes"] != float64(fits*len(listening)) {
		t.Errorf("the full library holds %v files and %v, want %d files and %d of audio", body["total"], usage, fits+1, fits*len(listening))
	}
	if removed := teacher.send(http.MethodDelete, "/teacher/media/"+clipID, nil); removed.status != http.StatusNoContent {
		t.Fatalf("deleting an unused audio file: %s", answer(removed))
	}
	if again := upload("", "bai-nghe.mp3", listening); again.status != http.StatusCreated {
		t.Errorf("an upload into the room a deleted file left: %s", answer(again))
	}
}
