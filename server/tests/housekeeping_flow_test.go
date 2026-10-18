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
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/core"
	"quizzivy/internal/core/maintenance"
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

func seedFinishedImport(t *testing.T, pool *pgxpool.Pool, owner, status string, ago time.Duration) string {
	t.Helper()
	id := uuid.NewString()
	var code *string
	created, outcome := 1, "created"
	if status == "failed" {
		failure := "WORKER_INTERRUPTED"
		code, created, outcome = &failure, 0, "pending"
	}
	if _, err := pool.Exec(context.Background(), `WITH job AS (
		INSERT INTO app.user_import_jobs (id, created_by, request_id, filename, format, default_role_id, delivery, status, total_rows, created_count, attempts,
		                                  error_code, started_at, completed_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 'housekeeping.csv', 'csv', (SELECT id FROM app.roles WHERE builtin_key = 'student'), 'email_link', $4, 1, $5, 1,
		        $6, now() - make_interval(hours => $7) - interval '1 hour', now() - make_interval(hours => $7))
		RETURNING id)
		INSERT INTO app.user_import_rows (job_id, ordinal, full_name, email, role_id, outcome)
		SELECT id, 0, 'Học viên', 'housekeeping-import-' || id || '@example.com', (SELECT id FROM app.roles WHERE builtin_key = 'student'), $8 FROM job`,
		id, owner, uuid.NewString(), status, created, code, int(ago.Hours()), outcome); err != nil {
		t.Fatal(err)
	}
	return id
}

func seedAnonymisedAccount(t *testing.T, pool *pgxpool.Pool, media *storage.Client) (id string, keys []string) {
	t.Helper()
	ctx := context.Background()
	if err := pool.QueryRow(ctx, `INSERT INTO app.users (email, full_name, role_id)
		VALUES ('housekeeping-anonymised-' || gen_random_uuid() || '@example.com', 'Học viên sắp ẩn danh', (SELECT id FROM app.roles WHERE builtin_key = 'student'))
		RETURNING id::text`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, sql := range []string{`DELETE FROM app.audit_log WHERE entity_id = $1::uuid`, `DELETE FROM app.users WHERE id = $1::uuid`} {
			if _, err := pool.Exec(ctx, sql, id); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		}
	})
	keys = []string{"avatars/" + id + "/" + uuid.NewString() + ".png", "avatars/" + id + "/" + uuid.NewString() + ".png"}
	if _, err := pool.Exec(ctx, `UPDATE app.users SET avatar_key = $2 WHERE id = $1::uuid`, id, keys[1]); err != nil {
		t.Fatal(err)
	}
	if media != nil {
		t.Cleanup(func() {
			for _, key := range keys {
				if err := media.Delete(ctx, key); err != nil {
					t.Errorf("cleanup %s: %v", key, err)
				}
			}
		})
		for _, key := range keys {
			body := []byte("photo " + key)
			if err := media.Put(ctx, key, "image/png", bytes.NewReader(body), int64(len(body))); err != nil {
				t.Fatal(err)
			}
		}
	}
	if report, err := maintenance.AnonymizeUser(ctx, pool, id, true); err != nil || !report.Applied {
		t.Fatalf("anonymise the account: %+v, %v", report, err)
	}
	return id, keys
}

func avatarSwept(t *testing.T, pool *pgxpool.Pool, id string) bool {
	t.Helper()
	var swept bool
	if err := pool.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM app.avatar_sweeps WHERE user_id = $1::uuid)`, id).Scan(&swept); err != nil {
		t.Fatal(err)
	}
	return swept
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

	anonymised, anonymisedPhotos := seedAnonymisedAccount(t, pool, media)
	livePhoto := "avatars/" + owner + "/" + uuid.NewString() + ".png"
	put(media, livePhoto)
	exec(`UPDATE app.users SET avatar_key = $2 WHERE id = $1::uuid`, owner, livePhoto)

	sum := sha256.Sum256([]byte("fresh orphan " + tag))
	freshLogo := "branding/" + hex.EncodeToString(sum[:]) + ".png"
	put(media, freshLogo)

	exec(`INSERT INTO app.user_activity_days (user_id, day) VALUES ($1::uuid, current_date - 450), ($1::uuid, current_date - 350), ($1::uuid, current_date)`, owner)
	sentOld, failedOld := outbox("sent-old", "sent", 40*24*time.Hour), outbox("failed-old", "failed", 40*24*time.Hour)
	sentRecent, failedRecent, pending := outbox("sent-recent", "sent", 5*24*time.Hour), outbox("failed-recent", "failed", 24*time.Hour), outbox("pending", "", 90*24*time.Hour)
	exec(`INSERT INTO app.set_password_tokens (user_id, token_hash, purpose, created_at, expires_at, used_at)
		VALUES ($1::uuid, sha256(convert_to('housekeeping-ended-' || $2, 'UTF8')), 'reset', now() - interval '41 days', now() - interval '40 days', NULL),
		       ($1::uuid, sha256(convert_to('housekeeping-used-' || $2, 'UTF8')), 'reset', now() - interval '6 days', now() - interval '5 days', now() - interval '130 hours'),
		       ($1::uuid, sha256(convert_to('housekeeping-live-' || $2, 'UTF8')), 'reset', now(), now() + interval '24 hours', NULL)`, owner, tag)
	sheet := func(expiresIn time.Duration) string {
		var jobID string
		exec(`WITH job AS (
			INSERT INTO app.user_import_jobs (created_by, request_id, filename, format, default_role_id, delivery, status, total_rows, created_count, attempts,
			                                  started_at, completed_at, sheet_expires_at)
			VALUES ($1::uuid, $2::uuid, 'housekeeping.csv', 'csv', (SELECT id FROM app.roles WHERE builtin_key = 'student'), 'sheet', 'done', 1, 1, 1,
			        now() - interval '30 hours', now() - interval '25 hours', now() + make_interval(hours => $3))
			RETURNING id)
			INSERT INTO app.user_import_rows (job_id, ordinal, full_name, email, role_id, outcome, secret_ciphertext)
			SELECT id, 0, 'Học viên', 'housekeeping-sheet-' || $2 || '@example.com', (SELECT id FROM app.roles WHERE builtin_key = 'student'), 'created', '\x0102'::bytea FROM job`,
			owner, uuid.NewString(), int(expiresIn.Hours()))
		if err := pool.QueryRow(ctx, `SELECT id::text FROM app.user_import_jobs WHERE created_by = $1::uuid ORDER BY created_at DESC, id DESC LIMIT 1`, owner).Scan(&jobID); err != nil {
			t.Fatal(err)
		}
		return jobID
	}
	lapsedSheet, liveSheet := sheet(-time.Hour), sheet(time.Hour)
	oldDone, oldFailed, recentDone := seedFinishedImport(t, pool, owner, "done", 31*24*time.Hour), seedFinishedImport(t, pool, owner, "failed", 31*24*time.Hour), seedFinishedImport(t, pool, owner, "done", 29*24*time.Hour)
	lapsedID, lapsedKey := export(9*24*time.Hour, -2*24*time.Hour)
	liveID, liveKey := export(time.Hour, 6*24*time.Hour)

	t.Cleanup(func() {
		for _, sql := range []string{
			`DELETE FROM app.email_outbox WHERE to_address LIKE $1`,
			`DELETE FROM app.data_exports WHERE requested_by = $1::uuid`,
			`DELETE FROM app.user_activity_days WHERE user_id = $1::uuid`,
			`DELETE FROM app.set_password_tokens WHERE user_id = $1::uuid`,
			`DELETE FROM app.user_import_jobs WHERE created_by = $1::uuid`,
		} {
			arg := owner
			if strings.Contains(sql, "email_outbox") {
				arg = "housekeeping-%-" + tag + "@example.com"
			}
			if _, err := pool.Exec(ctx, sql, arg); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		}
		for client, keys := range map[*storage.Client][]string{imports: {lapsedKey, liveKey}, media: {freshLogo, livePhoto}} {
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

	steps := []string{"activity_days", "email_outbox", "set_password_links", "import_sheets", "import_jobs", "data_exports", "avatar_objects", "branding_objects"}
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
	if got := count(`SELECT count(*) FROM app.set_password_tokens WHERE user_id = $1::uuid`, owner); got != 2 {
		t.Errorf("%d set-password links are left, want the one used 5 days ago and the live one", got)
	}
	if got := count(`SELECT count(*) FROM app.set_password_tokens WHERE user_id = $1::uuid AND expires_at < now() - interval '30 days'`, owner); got != 0 {
		t.Errorf("%d links that ended over 30 days ago are left", got)
	}
	sealedLeft := func(jobID string) int {
		return count(`SELECT count(*) FROM app.user_import_rows WHERE job_id = $1::uuid AND secret_ciphertext IS NOT NULL`, jobID)
	}
	if got := sealedLeft(lapsedSheet); got != 0 {
		t.Errorf("%d sealed password(s) of a sheet nobody took in its 24 hours are left", got)
	}
	if got := count(`SELECT count(*) FROM app.user_import_rows WHERE job_id = $1::uuid AND outcome = 'created'`, lapsedSheet); got != 1 {
		t.Errorf("the lapsed sheet's row was removed with its password: %d row(s) left, want 1", got)
	}
	if got := sealedLeft(liveSheet); got != 1 {
		t.Errorf("%d sealed password(s) of a sheet inside its 24 hours are left, want 1", got)
	}
	for name, c := range map[string]struct {
		id   string
		kept bool
	}{
		"import done 31 days ago":    {oldDone, false},
		"import failed 31 days ago":  {oldFailed, false},
		"import done 29 days ago":    {recentDone, true},
		"sheet import done 25 hours": {lapsedSheet, true},
	} {
		jobs := count(`SELECT count(*) FROM app.user_import_jobs WHERE id = $1::uuid`, c.id)
		rows := count(`SELECT count(*) FROM app.user_import_rows WHERE job_id = $1::uuid`, c.id)
		if kept := jobs == 1; kept != c.kept || rows != jobs {
			t.Errorf("the %s: %d job(s) and %d row(s) left, want %s", name, jobs, rows, map[bool]string{true: "both kept", false: "both gone"}[c.kept])
		}
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
	for _, key := range anonymisedPhotos {
		if present(media, key) {
			t.Errorf("a photo object of an anonymised account is left: %s", key)
		}
	}
	if !avatarSwept(t, pool, anonymised) {
		t.Error("the anonymised account is not marked as swept")
	}
	if !present(media, livePhoto) || avatarSwept(t, pool, owner) {
		t.Errorf("the photo of an account in use was touched: kept %v, marked %v", present(media, livePhoto), avatarSwept(t, pool, owner))
	}

	leaked := logs.String()
	for _, secret := range []string{lapsedID, liveID, lapsedKey, freshLogo, tag, owner, oldDone, oldFailed, anonymised, "avatars/"} {
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

	anonymised, _ := seedAnonymisedAccount(t, w.pool, nil)

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
	for !strings.Contains(logs.String(), `"step":"branding_objects"`) || !strings.Contains(logs.String(), `"step":"data_exports"`) || !strings.Contains(logs.String(), `"step":"avatar_objects"`) {
		if time.Now().After(deadline) {
			t.Fatalf("the first housekeeping run did not report the three storage steps: %s", logs.String())
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
	if got := strings.Count(text, `"code":"HOUSEKEEPING_STEP_FAILED"`); got != 3 {
		t.Errorf("%d steps failed, want the three that need storage: %s", got, text)
	}
	if !strings.Contains(text, `"failure":"network"`) {
		t.Errorf("the log does not name the class of the failure: %s", text)
	}
	for _, step := range []string{"activity_days", "email_outbox", "set_password_links", "import_sheets", "import_jobs"} {
		if !strings.Contains(text, `"msg":"housekeeping step done","step":"`+step+`"`) {
			t.Errorf("the step %s did not finish beside the failing ones: %s", step, text)
		}
	}
	for _, leak := range []string{id, key, "exports/", "branding/", "avatars/", anonymised, "127.0.0.1", away, served.S3Bucket, served.ImportBucket, owner, email} {
		if strings.Contains(text, leak) {
			t.Errorf("the log carries %q: %s", leak, text)
		}
	}
	var state string
	if err := w.pool.QueryRow(ctx, `SELECT status FROM app.data_exports WHERE id = $1::uuid`, id).Scan(&state); err != nil || state != "ready" {
		t.Errorf("the export is %q (%v), want it still ready for the next run", state, err)
	}
	if avatarSwept(t, w.pool, anonymised) {
		t.Error("an account whose photos could not be listed is marked as swept")
	}
}

func TestOldImportsAreRemovedOnAServerWhoseImportWorkerIsOff(t *testing.T) {
	ctx := context.Background()
	configure := func(c *config.Config) {
		c.ImportBucket = ""
		c.ImportProcessing = false
		c.MediaRetentionSweep = false
	}
	w := boot(t, configure)
	email, _ := w.createStaff("admin")
	var owner string
	if err := w.pool.QueryRow(ctx, `SELECT id::text FROM app.users WHERE email = $1`, email).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	old, recent := seedFinishedImport(t, w.pool, owner, "done", 31*24*time.Hour), seedFinishedImport(t, w.pool, owner, "done", 29*24*time.Hour)
	t.Cleanup(func() {
		if _, err := w.pool.Exec(ctx, `DELETE FROM app.user_import_jobs WHERE created_by = $1::uuid`, owner); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})

	served := e2eConfig(t, configure)
	if served.ImportProcessing {
		t.Fatal("the application under test has its import worker on")
	}
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
	for !strings.Contains(logs.String(), `"step":"import_jobs"`) {
		if time.Now().After(deadline) {
			t.Fatalf("the first housekeeping run did not report the import step: %s", logs.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
	count := func(id string) int {
		t.Helper()
		var n int
		if err := w.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM app.user_import_jobs WHERE id = $1::uuid) + (SELECT count(*) FROM app.user_import_rows WHERE job_id = $1::uuid)`, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if got := count(old); got != 0 {
		t.Errorf("an import finished 31 days ago left %d job and row(s)", got)
	}
	if got := count(recent); got != 2 {
		t.Errorf("an import finished 29 days ago left %d job and row(s), want both", got)
	}
	if strings.Contains(logs.String(), `"step":"import_sheets"`) {
		t.Errorf("the sheet step ran with no import worker: %s", logs.String())
	}
}

func TestWithoutAMediaBucketNoAnonymisedAccountIsMarked(t *testing.T) {
	ctx := context.Background()
	configure := func(c *config.Config) {
		c.S3Endpoint, c.S3Bucket = "", ""
		c.ImportBucket = ""
		c.ImportProcessing = false
		c.MediaRetentionSweep = false
	}
	w := boot(t, configure)
	anonymised, _ := seedAnonymisedAccount(t, w.pool, nil)

	served := e2eConfig(t, configure)
	if served.MediaEnabled() {
		t.Fatal("the application under test has a media bucket")
	}
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
	for !strings.Contains(logs.String(), `"step":"import_jobs"`) {
		if time.Now().After(deadline) {
			t.Fatalf("the first housekeeping run did not report the import step: %s", logs.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
	if strings.Contains(logs.String(), `"step":"avatar_objects"`) {
		t.Errorf("the avatar step ran with no media bucket: %s", logs.String())
	}
	if avatarSwept(t, w.pool, anonymised) {
		t.Error("an anonymised account was marked on a server that cannot reach its photos")
	}
}
