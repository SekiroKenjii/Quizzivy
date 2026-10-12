//go:build e2e

package e2e

import (
	"context"
	"log/slog"
	"quizzivy/internal/core"
	"quizzivy/internal/platform/config"
	"strings"
	"testing"
	"time"
)

func TestTheMediaSweepRunsWhileItsSwitchIsOnAndNeverWhenItIsOff(t *testing.T) {
	storage := func(c *config.Config) {
		c.S3Endpoint = storageEnv("S3_ENDPOINT", "http://localhost:9000")
		c.S3Region = storageEnv("S3_REGION", "us-east-1")
		c.S3Bucket = storageEnv("S3_BUCKET", "quizzivy-media")
		c.S3AccessKeyID = storageEnv("S3_ACCESS_KEY_ID", "quizzivy")
		c.S3SecretAccessKey = storageEnv("S3_SECRET_ACCESS_KEY", "quizzivy-dev-secret")
		c.S3ForcePathStyle = true
	}
	for name, on := range map[string]bool{"on": true, "off": false} {
		t.Run(name, func(t *testing.T) {
			w := boot(t, storage)
			email, _ := w.createStaff("teacher")
			var owner, asset string
			if err := w.pool.QueryRow(context.Background(), `SELECT id::text FROM app.users WHERE email=$1`, email).Scan(&owner); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				for _, sql := range []string{
					`DELETE FROM app.audit_log WHERE entity_id IN (SELECT id FROM app.media_assets WHERE owner_id=$1)`,
					`DELETE FROM app.media_assets WHERE owner_id=$1`,
				} {
					if _, err := w.pool.Exec(context.Background(), sql, owner); err != nil {
						t.Errorf("cleanup: %v", err)
					}
				}
			})
			if err := w.pool.QueryRow(context.Background(), `
				INSERT INTO app.media_assets (kind, storage_key, mime_type, bytes, original_filename, checksum_sha256, uploaded_by, owner_id, deleted_at, unreferenced_at)
				VALUES ('image', $1, 'image/png', 10, 'cu.png', sha256(convert_to($1,'UTF8')), $2, $2, now() - interval '9 days', now() - interval '8 days')
				RETURNING id::text`, "image/e2e-sweep-"+nonce(t)+".png", owner).Scan(&asset); err != nil {
				t.Fatal(err)
			}

			logs := &lockedBuffer{}
			cfg := e2eConfig(t, storage, func(c *config.Config) { c.MediaRetentionSweep = on })
			app, err := core.New(context.Background(), cfg, slog.New(slog.NewJSONHandler(logs, nil)))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(app.Close)
			ctx, stop := context.WithCancel(context.Background())
			served := make(chan error, 1)
			go func() { served <- app.Serve(ctx) }()
			t.Cleanup(func() {
				stop()
				<-served
			})

			purged := func() bool {
				var marked bool
				if err := w.pool.QueryRow(context.Background(), `SELECT purged_at IS NOT NULL FROM app.media_assets WHERE id=$1`, asset).Scan(&marked); err != nil {
					t.Fatal(err)
				}
				return marked
			}
			deadline := time.Now().Add(15 * time.Second)
			if on {
				for !purged() && time.Now().Before(deadline) {
					time.Sleep(50 * time.Millisecond)
				}
				if !purged() || strings.Contains(logs.String(), "media retention sweep is off") {
					t.Fatalf("with the switch on the due file was not removed: %s", logs.String())
				}
				return
			}
			time.Sleep(3 * time.Second)
			if purged() || strings.Contains(logs.String(), "media retention swept") {
				t.Fatalf("with the switch off the sweep removed a file: %s", logs.String())
			}
			if !strings.Contains(logs.String(), "media retention sweep is off") {
				t.Errorf("the switch being off is not logged: %s", logs.String())
			}
		})
	}
}
