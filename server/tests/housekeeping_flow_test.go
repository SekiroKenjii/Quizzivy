//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"quizzivy/internal/core"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/storage"
)

func housekeepingConfig(work string) func(*config.Config) {
	return func(c *config.Config) {
		c.S3Endpoint = storageEnv("S3_ENDPOINT", "http://localhost:9000")
		c.S3Region = storageEnv("S3_REGION", "us-east-1")
		c.S3Bucket = storageEnv("S3_BUCKET", "quizzivy-media")
		c.S3AccessKeyID = storageEnv("S3_ACCESS_KEY_ID", "quizzivy")
		c.S3SecretAccessKey = storageEnv("S3_SECRET_ACCESS_KEY", "quizzivy-dev-secret")
		c.S3ForcePathStyle = true
		c.SignedURLTTL = time.Minute
		c.ImportBucket = storageEnv("IMPORT_S3_BUCKET", "quizzivy-imports")
		c.ImportWorkDir = work
		c.ImportActorCount, c.ImportGlobalCount, c.ImportSourcesPerItem = 100, 1000, 32
		c.ImportActorMiB, c.ImportGlobalMiB = 256, 1024
		c.ImportProcessing = true
		c.MediaRetentionSweep = false
	}
}

func TestTheDailyHousekeepingClearsWhatOutlivedItsKeepingAndNothingElse(t *testing.T) {
	ctx := context.Background()
	configure := housekeepingConfig(t.TempDir())
	w := boot(t, configure)
	cfg, pool := w.cfg, w.pool

	email, _ := w.createStaff("admin")
	var owner string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM app.users WHERE email = $1`, email).Scan(&owner); err != nil {
		t.Fatal(err)
	}

	bucket := func(name string) *storage.Client {
		client, err := storage.New(ctx, storage.Config{Endpoint: cfg.S3Endpoint, Region: cfg.S3Region, Bucket: name, AccessKeyID: cfg.S3AccessKeyID, SecretAccessKey: cfg.S3SecretAccessKey, ForcePathStyle: true})
		if err != nil {
			t.Fatal(err)
		}
		return client
	}
	media, imports := bucket(cfg.S3Bucket), bucket(cfg.ImportBucket)
	put := func(client *storage.Client, key string) {
		t.Helper()
		body := []byte("housekeeping " + key)
		if err := client.Put(ctx, key, "application/octet-stream", bytes.NewReader(body), int64(len(body))); err != nil {
			t.Fatal(err)
		}
	}
	present := func(client *storage.Client, key string) bool {
		t.Helper()
		found, err := client.Exists(ctx, key)
		if err != nil {
			t.Fatal(err)
		}
		return found
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}

	tag := nonce(t)
	address := func(name string) string { return "housekeeping-" + name + "-" + tag + "@example.com" }
	outbox := func(name, outcome string, ago time.Duration) string {
		var id string
		column := map[string]string{"sent": "sent_at", "failed": "failed_at"}[outcome]
		sql := `INSERT INTO app.email_outbox (to_address, template, locale, deliver_after, created_at) VALUES ($1, 'housekeeping', 'vi', now() + interval '1 year', now() - make_interval(hours => $2)) RETURNING id::text`
		if column != "" {
			sql = `INSERT INTO app.email_outbox (to_address, template, locale, deliver_after, created_at, ` + column + `) VALUES ($1, 'housekeeping', 'vi', now() + interval '1 year', now() - make_interval(hours => $2), now() - make_interval(hours => $2)) RETURNING id::text`
		}
		if err := pool.QueryRow(ctx, sql, address(name), int(ago.Hours())).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	export := func(ago, expiresIn time.Duration) (id, key string) {
		id = uuid.NewString()
		key = "exports/" + id + ".zip"
		put(imports, key)
		exec(`INSERT INTO app.data_exports (id, requested_by, include_answers, status, attempts, storage_key, bytes, created_at, started_at, completed_at, expires_at)
			VALUES ($1::uuid, $2::uuid, false, 'ready', 1, $3, 10, now() - make_interval(hours => $4), now() - make_interval(hours => $4), now() - make_interval(hours => $4), now() + make_interval(hours => $5))`,
			id, owner, key, int(ago.Hours()), int(expiresIn.Hours()))
		return id, key
	}

	sum := sha256.Sum256([]byte("fresh orphan " + tag))
	freshLogo := "branding/" + hex.EncodeToString(sum[:]) + ".png"
	put(media, freshLogo)

	exec(`INSERT INTO app.user_activity_days (user_id, day) VALUES ($1::uuid, current_date - 450), ($1::uuid, current_date - 350), ($1::uuid, current_date)`, owner)
	sentOld, failedOld := outbox("sent-old", "sent", 40*24*time.Hour), outbox("failed-old", "failed", 40*24*time.Hour)
	sentRecent, failedRecent, pending := outbox("sent-recent", "sent", 5*24*time.Hour), outbox("failed-recent", "failed", 24*time.Hour), outbox("pending", "", 90*24*time.Hour)
	lapsedID, lapsedKey := export(9*24*time.Hour, -2*24*time.Hour)
	liveID, liveKey := export(time.Hour, 6*24*time.Hour)

	t.Cleanup(func() {
		for _, sql := range []string{
			`DELETE FROM app.email_outbox WHERE to_address LIKE $1`,
			`DELETE FROM app.data_exports WHERE requested_by = $1::uuid`,
			`DELETE FROM app.user_activity_days WHERE user_id = $1::uuid`,
		} {
			arg := owner
			if strings.Contains(sql, "email_outbox") {
				arg = "housekeeping-%-" + tag + "@example.com"
			}
			if _, err := pool.Exec(ctx, sql, arg); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		}
		for client, keys := range map[*storage.Client][]string{imports: {lapsedKey, liveKey}, media: {freshLogo}} {
			for _, key := range keys {
				if err := client.Delete(ctx, key); err != nil {
					t.Errorf("cleanup %s: %v", key, err)
				}
			}
		}
	})

	logs := &lockedBuffer{}
	served := e2eConfig(t, configure)
	if served.S3Bucket != cfg.S3Bucket || served.ImportBucket != cfg.ImportBucket || served.DatabaseURL != cfg.DatabaseURL {
		t.Fatalf("the application under test sits on other storage than the rows seeded: %+v", served)
	}
	app, err := core.New(ctx, served, slog.New(slog.NewJSONHandler(logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Close)
	serving, stop := context.WithCancel(ctx)
	stopped := make(chan error, 1)
	go func() { stopped <- app.Serve(serving) }()
	t.Cleanup(func() {
		stop()
		<-stopped
	})

	steps := []string{"activity_days", "email_outbox", "data_exports", "branding_objects"}
	deadline := time.Now().Add(30 * time.Second)
	for {
		seen := true
		for _, step := range steps {
			seen = seen && strings.Contains(logs.String(), `"step":"`+step+`"`)
		}
		if seen {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the first housekeeping run did not report every step: %s", logs.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
	if strings.Contains(logs.String(), "HOUSEKEEPING_STEP_FAILED") {
		t.Fatalf("a housekeeping step failed: %s", logs.String())
	}

	count := func(sql string, args ...any) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if got := count(`SELECT count(*) FROM app.user_activity_days WHERE user_id = $1::uuid`, owner); got != 2 {
		t.Errorf("%d activity days are left, want the two inside the 400 days", got)
	}
	if got := count(`SELECT count(*) FROM app.user_activity_days WHERE user_id = $1::uuid AND day < current_date - 400`, owner); got != 0 {
		t.Errorf("%d activity days older than 400 days are left", got)
	}
	for name, c := range map[string]struct {
		id   string
		kept bool
	}{
		"sent 40 days ago":    {sentOld, false},
		"failed 40 days ago":  {failedOld, false},
		"sent 5 days ago":     {sentRecent, true},
		"failed 1 day ago":    {failedRecent, true},
		"pending for 90 days": {pending, true},
	} {
		if kept := count(`SELECT count(*) FROM app.email_outbox WHERE id = $1::uuid`, c.id) == 1; kept != c.kept {
			t.Errorf("the outbox row %s: kept = %v, want %v", name, kept, c.kept)
		}
	}
	status := func(id string) (state string, hasKey bool) {
		t.Helper()
		if err := pool.QueryRow(ctx, `SELECT status, storage_key IS NOT NULL FROM app.data_exports WHERE id = $1::uuid`, id).Scan(&state, &hasKey); err != nil {
			t.Fatal(err)
		}
		return state, hasKey
	}
	if state, hasKey := status(lapsedID); state != "expired" || hasKey || present(imports, lapsedKey) {
		t.Errorf("the lapsed export is %q (key kept: %v, file kept: %v), want expired with no file", state, hasKey, present(imports, lapsedKey))
	}
	if state, hasKey := status(liveID); state != "ready" || !hasKey || !present(imports, liveKey) {
		t.Errorf("the export inside its time is %q (key kept: %v, file kept: %v), want ready with its file", state, hasKey, present(imports, liveKey))
	}
	if !present(media, freshLogo) {
		t.Error("a logo object written a moment ago was removed inside its 24 hour grace")
	}

	leaked := logs.String()
	for _, secret := range []string{lapsedID, liveID, lapsedKey, freshLogo, tag, owner} {
		if strings.Contains(leaked, secret) {
			t.Errorf("the housekeeping logs carry %q", secret)
		}
	}
}

func TestAStepThatCannotReachStorageLogsItsClassAndNoKeyEndpointOrBucket(t *testing.T) {
	ctx := context.Background()
	configure := housekeepingConfig(t.TempDir())
	w := boot(t, configure)
	email, _ := w.createStaff("admin")
	var owner string
	if err := w.pool.QueryRow(ctx, `SELECT id::text FROM app.users WHERE email = $1`, email).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	id := uuid.NewString()
	key := "exports/" + id + ".zip"
	if _, err := w.pool.Exec(ctx, `INSERT INTO app.data_exports (id, requested_by, include_answers, status, attempts, storage_key, bytes, created_at, started_at, completed_at, expires_at)
		VALUES ($1::uuid, $2::uuid, false, 'ready', 1, $3, 10, now() - interval '9 days', now() - interval '9 days', now() - interval '9 days', now() - interval '2 days')`, id, owner, key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := w.pool.Exec(ctx, `DELETE FROM app.data_exports WHERE requested_by = $1::uuid`, owner); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})

	served := e2eConfig(t, configure)
	const away = "http://127.0.0.1:1"
	served.S3Endpoint = away
	logs := &lockedBuffer{}
	app, err := core.New(ctx, served, slog.New(slog.NewJSONHandler(logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Close)
	serving, stop := context.WithCancel(ctx)
	stopped := make(chan error, 1)
	go func() { stopped <- app.Serve(serving) }()
	t.Cleanup(func() {
		stop()
		<-stopped
	})

	deadline := time.Now().Add(30 * time.Second)
	for !strings.Contains(logs.String(), `"step":"branding_objects"`) || !strings.Contains(logs.String(), `"step":"data_exports"`) {
		if time.Now().After(deadline) {
			t.Fatalf("the first housekeeping run did not report both storage steps: %s", logs.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
	var lines []string
	for _, line := range strings.Split(logs.String(), "\n") {
		if strings.Contains(line, `"msg":"housekeeping`) {
			lines = append(lines, line)
		}
	}
	text := strings.Join(lines, "\n")
	if got := strings.Count(text, `"code":"HOUSEKEEPING_STEP_FAILED"`); got != 2 {
		t.Errorf("%d steps failed, want the two that need storage: %s", got, text)
	}
	if !strings.Contains(text, `"failure":"network"`) {
		t.Errorf("the log does not name the class of the failure: %s", text)
	}
	for _, step := range []string{"activity_days", "email_outbox"} {
		if !strings.Contains(text, `"msg":"housekeeping step done","step":"`+step+`"`) {
			t.Errorf("the step %s did not finish beside the failing ones: %s", step, text)
		}
	}
	for _, leak := range []string{id, key, "exports/", "branding/", "127.0.0.1", away, served.S3Bucket, served.ImportBucket, owner, email} {
		if strings.Contains(text, leak) {
			t.Errorf("the log carries %q: %s", leak, text)
		}
	}
	var state string
	if err := w.pool.QueryRow(ctx, `SELECT status FROM app.data_exports WHERE id = $1::uuid`, id).Scan(&state); err != nil || state != "ready" {
		t.Errorf("the export is %q (%v), want it still ready for the next run", state, err)
	}
}
