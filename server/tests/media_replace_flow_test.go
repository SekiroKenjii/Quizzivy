//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"quizzivy/internal/platform/config"
	"testing"
	"time"
)

func TestMediaReplacementWholeApplicationRetainsOwnerAndOldObject(t *testing.T) {
	w := boot(t, func(c *config.Config) {
		c.S3Endpoint = storageEnv("S3_ENDPOINT", "http://localhost:9000")
		c.S3Region = storageEnv("S3_REGION", "us-east-1")
		c.S3Bucket = storageEnv("S3_BUCKET", "quizzivy-media")
		c.S3AccessKeyID = storageEnv("S3_ACCESS_KEY_ID", "quizzivy")
		c.S3SecretAccessKey = storageEnv("S3_SECRET_ACCESS_KEY", "quizzivy-dev-secret")
		c.S3ForcePathStyle = true
		c.SignedURLTTL = time.Minute
	})
	email, password := w.createStaff("teacher")
	teacher := w.signedIn(email, password)
	drawing := tinyPNG(t)
	old := replacementSend(t, teacher, http.StatusCreated, "/teacher/media", new(filePayload(t, "gốc.png", drawing)))
	teacher.must(http.StatusOK, http.MethodPatch, "/teacher/media/"+id(old), map[string]any{"displayName": "Tên được giữ"})
	question := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/questions", map[string]any{"type": "single_choice", "prompt": "Chọn", "points": 1, "mediaAssetId": id(old), "options": []map[string]any{{"text": "Đúng", "isCorrect": true}, {"text": "Sai", "isCorrect": false}}})
	adminEmail, adminPassword := w.createStaff("admin")
	admin := w.signedIn(adminEmail, adminPassword)
	result := replacementSend(t, admin, http.StatusCreated, "/teacher/media/"+id(old)+"/replace", new(filePayload(t, "mới.png", drawing)))
	if len(result) != 3 {
		t.Fatal("replacement envelope has unexpected fields")
	}
	asset := result["asset"].(map[string]any)
	repointed := result["repointed"].(map[string]any)
	left := result["left"].(map[string]any)
	if id(asset) == id(old) || asset["displayName"] != "Tên được giữ" || asset["originalFilename"] != "mới.png" || repointed["questions"] != float64(1) || repointed["groups"] != float64(0) || left["questions"] != float64(0) || left["groups"] != float64(0) || len(repointed) != 2 || len(left) != 2 {
		t.Fatal("replacement identity, metadata or counts differ")
	}
	if updated := teacher.must(http.StatusOK, http.MethodGet, "/teacher/questions/"+id(question), nil); id(updated["media"].(map[string]any)) != id(asset) {
		t.Fatal("question did not expose the replacement media identity")
	}
	var owner, uploader, teacherID, adminID string
	if err := w.pool.QueryRow(context.Background(), `SELECT owner_id::text,uploaded_by::text FROM app.media_assets WHERE id=$1`, id(asset)).Scan(&owner, &uploader); err != nil {
		t.Fatal(err)
	}
	if err := w.pool.QueryRow(context.Background(), `SELECT id::text FROM app.users WHERE email=$1`, email).Scan(&teacherID); err != nil {
		t.Fatal(err)
	}
	if err := w.pool.QueryRow(context.Background(), `SELECT id::text FROM app.users WHERE email=$1`, adminEmail).Scan(&adminID); err != nil {
		t.Fatal(err)
	}
	if owner != teacherID || uploader != adminID {
		t.Fatalf("owner/uploader=%s/%s", owner, uploader)
	}
	list := teacher.must(http.StatusOK, http.MethodGet, "/teacher/media", nil)
	rows := list["items"].([]any)
	if len(rows) != 1 || id(rows[0].(map[string]any)) != id(asset) {
		t.Fatal("owner library did not contain exactly the replacement")
	}
	replacementSend(t, teacher, http.StatusNotFound, "/teacher/media/"+id(old)+"/replace", new(filePayload(t, "lặp.png", drawing)))
	for _, row := range []map[string]any{old, asset} {
		response, err := http.Get(row["url"].(string))
		if err != nil {
			t.Fatal("get private object")
		}
		body, err := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if err != nil || closeErr != nil || response.StatusCode != 200 || !bytes.Equal(body, drawing) {
			t.Fatalf("immutable bytes/status=%d read=%v close=%v", response.StatusCode, err, closeErr)
		}
	}
}

func replacementSend(t *testing.T, c *client, want int, path string, body *payload) map[string]any {
	t.Helper()
	out := c.send(http.MethodPost, path, body)
	if out.status != want {
		t.Fatalf("replacement multipart status=%s want=%d", answer(out), want)
	}
	return out.json
}
