//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"reflect"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"quizzivy/internal/platform/config"
)

type fixtureInventory struct {
	Tables  map[string][]string `json:"tables"`
	Roles   []string            `json:"roles"`
	Buckets map[string][]string `json:"buckets"`
}

func fixtureStorageConfig() config.Config {
	return config.Config{S3Endpoint: storageEnv("S3_ENDPOINT", "http://localhost:9000"), S3Region: storageEnv("S3_REGION", "us-east-1"), S3Bucket: storageEnv("S3_BUCKET", "quizzivy-media"), ImportBucket: storageEnv("IMPORT_S3_BUCKET", "quizzivy-imports"), S3AccessKeyID: storageEnv("S3_ACCESS_KEY_ID", "quizzivy"), S3SecretAccessKey: storageEnv("S3_SECRET_ACCESS_KEY", "quizzivy-dev-secret"), S3ForcePathStyle: true}
}

func readFixtureInventory(ctx context.Context, conn *pgx.Conn) (fixtureInventory, error) {
	out := fixtureInventory{Tables: map[string][]string{}, Buckets: map[string][]string{}}
	rows, err := conn.Query(ctx, `SELECT n.nspname,c.relname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE (n.nspname='app' AND c.relkind IN ('r','p')) OR (n.nspname='public' AND c.relname='goose_db_version') ORDER BY n.nspname,c.relname`)
	if err != nil {
		return out, err
	}
	var tables [][2]string
	for rows.Next() {
		var table [2]string
		if err := rows.Scan(&table[0], &table[1]); err != nil {
			rows.Close()
			return out, err
		}
		tables = append(tables, table)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	for _, table := range tables {
		name := table[0] + "." + table[1]
		records, err := conn.Query(ctx, "SELECT to_jsonb(row)::text FROM "+pgx.Identifier{table[0], table[1]}.Sanitize()+" row ORDER BY to_jsonb(row)::text")
		if err != nil {
			return out, err
		}
		out.Tables[name] = []string{}
		for records.Next() {
			var row string
			if err := records.Scan(&row); err != nil {
				records.Close()
				return out, err
			}
			out.Tables[name] = append(out.Tables[name], row)
		}
		err = records.Err()
		records.Close()
		if err != nil {
			return out, err
		}
	}
	roles, err := conn.Query(ctx, `SELECT jsonb_build_array(rolname,rolsuper,rolcreatedb,rolcreaterole,rolbypassrls)::text FROM pg_roles ORDER BY rolname`)
	if err != nil {
		return out, err
	}
	for roles.Next() {
		var row string
		if err := roles.Scan(&row); err != nil {
			roles.Close()
			return out, err
		}
		out.Roles = append(out.Roles, row)
	}
	err = roles.Err()
	roles.Close()
	if err != nil {
		return out, err
	}
	cfg := fixtureStorageConfig()
	client, err := fixtureS3(ctx, cfg)
	if err != nil {
		return out, err
	}
	for _, name := range []string{cfg.S3Bucket, cfg.ImportBucket} {
		if _, exists := out.Buckets[name]; exists {
			continue
		}
		inventory, err := fixtureBucketInventory(ctx, client, name)
		if err != nil {
			return out, err
		}
		out.Buckets[name] = inventory
	}
	return out, nil
}

func fixtureBucketInventory(ctx context.Context, client *s3.Client, name string) ([]string, error) {
	result := []string{}
	status, err := client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: aws.String(name)})
	if err != nil {
		return nil, err
	}
	result = append(result, "versioning="+string(status.Status)+"/"+string(status.MFADelete))
	objects := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{Bucket: aws.String(name)})
	for objects.HasMorePages() {
		page, err := objects.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, v := range page.Contents {
			raw, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			result = append(result, "object="+string(raw))
		}
	}
	versions := s3.NewListObjectVersionsPaginator(client, &s3.ListObjectVersionsInput{Bucket: aws.String(name)})
	for versions.HasMorePages() {
		page, err := versions.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, v := range page.Versions {
			raw, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			result = append(result, "version="+string(raw))
		}
		for _, v := range page.DeleteMarkers {
			raw, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			result = append(result, "marker="+string(raw))
		}
	}
	sort.Strings(result)
	return result, nil
}

func writeFixtureInventory(path string, inventory fixtureInventory) error {
	raw, err := json.Marshal(inventory)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(raw)
	return errors.Join(writeErr, file.Close())
}

func lifecycleChild(t *testing.T, mode string) (*exec.Cmd, *bytes.Buffer) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=^TestFixtureLifecycleChild$", "-test.v", "-test.timeout=2m")
	env := make([]string, 0, len(os.Environ()))
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "TEST_DATABASE_URL=") && !strings.HasPrefix(value, "QUIZZIVY_E2E_") {
			env = append(env, value)
		}
	}
	cmd.Env = append(env, "TEST_DATABASE_URL="+fixtures.originalDSN, "QUIZZIVY_E2E_PROBE="+mode)
	output := new(bytes.Buffer)
	cmd.Stdout = output
	cmd.Stderr = output
	return cmd, output
}

func lifecycleResources(t *testing.T, output string) []fixtureResource {
	t.Helper()
	var resources []fixtureResource
	for _, line := range strings.Split(output, "\n") {
		if raw, found := strings.CutPrefix(line, "E2E_RESOURCE "); found {
			var resource fixtureResource
			if err := json.Unmarshal([]byte(raw), &resource); err != nil {
				t.Fatal(err)
			}
			resources = append(resources, resource)
		}
	}
	if len(resources) == 0 {
		t.Fatalf("child registered no resources: %s", output)
	}
	return resources
}

func lifecycleControl(t *testing.T) *pgx.Conn {
	t.Helper()
	if fixtures == nil {
		t.Skip("TEST_DATABASE_URL is not set; lifecycle verification needs PostgreSQL 18 and MinIO")
	}
	conn, err := pgx.Connect(context.Background(), fixtures.originalDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return conn
}

func requireFixtureAbsent(t *testing.T, conn *pgx.Conn, resources []fixtureResource) {
	t.Helper()
	client, err := fixtureS3(context.Background(), fixtureStorageConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, resource := range resources {
		switch resource.Kind {
		case "database":
			var exists bool
			if err := conn.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)`, resource.Name).Scan(&exists); err != nil {
				t.Error(err)
				continue
			}
			if exists {
				t.Errorf("owned child database remains: %s", resource.Name)
				recoverFixtureDatabase(t, conn, resource.Name)
			}
		case "bucket":
			_, err := client.HeadBucket(context.Background(), &s3.HeadBucketInput{Bucket: aws.String(resource.Name)})
			if !fixtureNotFound(err) {
				t.Errorf("owned child bucket remains or absence is unproven: %s (%v)", resource.Name, err)
				if err == nil {
					recoverFixtureBucket(t, client, resource.Name)
				}
			}
		default:
			t.Errorf("unknown fixture resource kind %q", resource.Kind)
		}
	}
}

func recoverFixtureDatabase(t *testing.T, conn *pgx.Conn, name string) {
	t.Helper()
	if _, err := conn.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Errorf("recover exact child-owned database %s: %v", name, err)
	}
}

func recoverFixtureBucket(t *testing.T, client *s3.Client, name string) {
	t.Helper()
	var versionObjects []types.ObjectIdentifier
	versions := s3.NewListObjectVersionsPaginator(client, &s3.ListObjectVersionsInput{Bucket: aws.String(name)})
	for versions.HasMorePages() {
		page, err := versions.NextPage(context.Background())
		if err != nil {
			t.Error(err)
			return
		}
		for _, v := range page.Versions {
			versionObjects = append(versionObjects, types.ObjectIdentifier{Key: v.Key, VersionId: v.VersionId})
		}
		for _, v := range page.DeleteMarkers {
			versionObjects = append(versionObjects, types.ObjectIdentifier{Key: v.Key, VersionId: v.VersionId})
		}
	}
	for len(versionObjects) > 0 {
		count := min(len(versionObjects), 1000)
		out, err := client.DeleteObjects(context.Background(), &s3.DeleteObjectsInput{Bucket: aws.String(name), Delete: &types.Delete{Objects: versionObjects[:count]}})
		if err != nil {
			t.Error(err)
			return
		}
		if len(out.Errors) > 0 {
			t.Errorf("recover child versions: %v", out.Errors)
			return
		}
		versionObjects = versionObjects[count:]
	}
	objects := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{Bucket: aws.String(name)})
	for objects.HasMorePages() {
		page, err := objects.NextPage(context.Background())
		if err != nil {
			t.Error(err)
			return
		}
		var batch []types.ObjectIdentifier
		for _, v := range page.Contents {
			batch = append(batch, types.ObjectIdentifier{Key: v.Key})
		}
		if len(batch) > 0 {
			out, err := client.DeleteObjects(context.Background(), &s3.DeleteObjectsInput{Bucket: aws.String(name), Delete: &types.Delete{Objects: batch}})
			if err != nil {
				t.Error(err)
				return
			}
			if len(out.Errors) > 0 {
				t.Errorf("recover child objects: %v", out.Errors)
				return
			}
		}
	}
	if _, err := client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(name)}); err != nil {
		t.Error(err)
	}
}

func TestFixtureLifecycleCleansCommittedFailureAndPartialSetup(t *testing.T) {
	conn := lifecycleControl(t)
	before, err := readFixtureInventory(context.Background(), conn)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"committed-failure", "after-database", "after-bucket", "cleanup-error", "object-delete-error"} {
		t.Run(mode, func(t *testing.T) {
			cmd, output := lifecycleChild(t, mode)
			err := cmd.Run()
			if err == nil {
				t.Error("deliberate failure exited successfully")
			}
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 {
				t.Errorf("failure exit=%v", err)
			}
			if mode == "committed-failure" && !strings.Contains(output.String(), "deliberate failure after committed login, media and source") {
				t.Errorf("the deliberate committed failure was not reached: %s", output.String())
			}
			resources := lifecycleResources(t, output.String())
			want := 1
			if mode == "after-bucket" || mode == "object-delete-error" {
				want = 2
			}
			if mode == "committed-failure" {
				want = 3
			}
			if len(resources) != want {
				t.Errorf("registered%d resources, want%d: %s", len(resources), want, output.String())
			}
			requireFixtureAbsent(t, conn, resources)
			t.Log(output.String())
		})
	}
	after, err := readFixtureInventory(context.Background(), conn)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Error("child failure paths changed the parent's full row/ID, role or object/version inventory")
	}
}

func TestFixtureLifecyclePreservesOptionalStoresAndSharedBoots(t *testing.T) {
	conn := lifecycleControl(t)
	for _, mode := range []string{"optional", "multi-boot", "same-store", "no-external-wake", "paging", "versioning"} {
		t.Run(mode, func(t *testing.T) {
			cmd, output := lifecycleChild(t, mode)
			if err := cmd.Run(); err != nil {
				t.Errorf("child %s: %v: %s", mode, err, output.String())
			}
			resources := lifecycleResources(t, output.String())
			requireFixtureAbsent(t, conn, resources)
			want := 3
			if mode == "optional" || mode == "multi-boot" {
				want = 1
			}
			if mode == "same-store" || mode == "paging" || mode == "versioning" {
				want = 2
			}
			if len(resources) != want {
				t.Errorf("%s registered%d resources, want%d", mode, len(resources), want)
			}
			t.Log(output.String())
		})
	}
}

func childStorage(t *testing.T, configure func(*config.Config)) *world {
	t.Helper()
	work := t.TempDir()
	if err := os.Chmod(work, 0700); err != nil {
		t.Fatal(err)
	}
	return boot(t, func(cfg *config.Config) {
		stores := fixtureStorageConfig()
		cfg.S3Endpoint = stores.S3Endpoint
		cfg.S3Region = stores.S3Region
		cfg.S3Bucket = stores.S3Bucket
		cfg.ImportBucket = stores.ImportBucket
		cfg.S3AccessKeyID = stores.S3AccessKeyID
		cfg.S3SecretAccessKey = stores.S3SecretAccessKey
		cfg.S3ForcePathStyle = true
		cfg.SignedURLTTL = 10 * time.Minute
		cfg.ImportWorkDir = work
		cfg.ImportActorCount = 100
		cfg.ImportGlobalCount = 1000
		cfg.ImportSourcesPerItem = 32
		cfg.ImportActorMiB = 256
		cfg.ImportGlobalMiB = 1024
		cfg.ImportProcessing = true
		if configure != nil {
			configure(cfg)
		}
	})
}

func childCommit(t *testing.T, w *world) *client {
	t.Helper()
	email, password := w.createStaff("teacher")
	teacher := w.signedIn(email, password)
	picture := teacher.send(http.MethodPost, "/teacher/media", new(filePayload(t, "lifecycle.png", tinyPNG(t))))
	if picture.status != http.StatusCreated {
		t.Fatalf("committed media status%d", picture.status)
	}
	imported := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/imports", map[string]any{"requestId": uuid.NewString(), "title": "Lifecycle"})
	source := teacher.send(http.MethodPost, "/teacher/imports/"+id(imported)+"/sources?role=exam&uploadId="+uuid.NewString()+"&expectedRevision="+jsonNumber(imported["revision"]), new(filePayload(t, "lifecycle.docx", tinyDocx(t))))
	if source.status != http.StatusCreated && source.status != http.StatusOK {
		t.Fatalf("committed source status%d", source.status)
	}
	var database string
	if err := w.pool.QueryRow(context.Background(), `SELECT current_database()`).Scan(&database); err != nil {
		t.Fatal(err)
	}
	if database != fixtures.database {
		t.Fatalf("fixture pool targets %s instead of the owned database", database)
	}
	var users, media, sources int
	if err := w.pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM app.users),(SELECT count(*) FROM app.media_assets),(SELECT count(*) FROM app.word_import_sources)`).Scan(&users, &media, &sources); err != nil {
		t.Fatal(err)
	}
	if users < 1 || media < 1 || sources < 1 {
		t.Fatalf("committed graph counts=%d/%d/%d", users, media, sources)
	}
	return teacher
}

func childBulkObjects(t *testing.T, mode string) {
	t.Helper()
	childStorage(t, func(cfg *config.Config) { cfg.ImportBucket = ""; cfg.ImportProcessing = false })
	fixtures.mu.Lock()
	bucket := fixtures.buckets[0]
	fixtures.mu.Unlock()
	if mode == "versioning" {
		if _, err := bucket.client.PutBucketVersioning(context.Background(), &s3.PutBucketVersioningInput{Bucket: aws.String(bucket.name), VersioningConfiguration: &types.VersioningConfiguration{Status: types.BucketVersioningStatusEnabled}}); err != nil {
			t.Fatal(err)
		}
	}
	count := 1005
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("page/%04d", i)
		if mode == "versioning" {
			key = "versioned"
		}
		if _, err := bucket.client.PutObject(context.Background(), &s3.PutObjectInput{Bucket: aws.String(bucket.name), Key: aws.String(key), Body: strings.NewReader("fixture")}); err != nil {
			t.Fatal(err)
		}
	}
	key := "page/0000"
	if mode == "versioning" {
		key = "versioned"
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, bucket.endpoint+"/"+bucket.name+"/"+key, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("owned bucket allowed anonymous read: status%d", response.StatusCode)
	}
	if mode == "versioning" {
		if _, err := bucket.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{Bucket: aws.String(bucket.name), Key: aws.String("versioned")}); err != nil {
			t.Fatal(err)
		}
	}
}

type fixtureDeleteErrorClient struct {
	delegate s3.HTTPClient
}

func (c fixtureDeleteErrorClient) Do(request *http.Request) (*http.Response, error) {
	response, err := c.delegate.Do(request)
	if err != nil || request.Method != http.MethodPost || !request.URL.Query().Has("delete") {
		return response, err
	}
	if err := response.Body.Close(); err != nil {
		return nil, err
	}
	body := `<DeleteResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Error><Key>fixture</Key><Code>AccessDenied</Code></Error></DeleteResult>`
	response.Body = io.NopCloser(strings.NewReader(body))
	response.ContentLength = int64(len(body))
	response.Header.Set("Content-Length", fmt.Sprint(len(body)))
	return response, nil
}

func TestFixtureLifecycleChild(t *testing.T) {
	mode := os.Getenv("QUIZZIVY_E2E_PROBE")
	if mode == "" {
		return
	}
	if mode == "cleanup-error" {
		return
	}
	if mode == "object-delete-error" {
		childStorage(t, func(cfg *config.Config) { cfg.ImportBucket = ""; cfg.ImportProcessing = false })
		bucket := fixtures.buckets[0]
		if _, err := bucket.client.PutObject(context.Background(), &s3.PutObjectInput{Bucket: aws.String(bucket.name), Key: aws.String("fixture"), Body: strings.NewReader("owned")}); err != nil {
			t.Fatal(err)
		}
		options := bucket.client.Options()
		options.HTTPClient = fixtureDeleteErrorClient{delegate: options.HTTPClient}
		bucket.client = s3.New(options)
		return
	}
	if mode == "after-bucket" {
		bootWithStorage(t)
		return
	}
	if mode == "optional" {
		w := boot(t)
		email, password := w.createStaff("teacher")
		teacher := w.signedIn(email, password)
		if caps := teacher.must(http.StatusOK, http.MethodGet, "/teacher/imports/capabilities", nil); caps["intakeEnabled"] != false {
			t.Fatal("unconfigured import store was enabled")
		}
		if len(fixtures.buckets) != 0 {
			t.Fatal("unconfigured stores created buckets")
		}
		return
	}
	if mode == "multi-boot" {
		first := boot(t)
		email, password := first.createStaff("teacher")
		second := boot(t)
		second.signedIn(email, password)
		var firstDB, secondDB string
		if err := first.pool.QueryRow(context.Background(), `SELECT current_database()`).Scan(&firstDB); err != nil {
			t.Fatal(err)
		}
		if err := second.pool.QueryRow(context.Background(), `SELECT current_database()`).Scan(&secondDB); err != nil {
			t.Fatal(err)
		}
		if firstDB != secondDB || firstDB != fixtures.database {
			t.Fatal("boot calls did not share the owned database")
		}
		return
	}
	if mode == "paging" || mode == "versioning" {
		childBulkObjects(t, mode)
		return
	}
	var externalCalls atomic.Int64
	trap := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		externalCalls.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer trap.Close()
	w := childStorage(t, func(cfg *config.Config) {
		cfg.ImportWorkerWakeURL = trap.URL
		if mode == "same-store" {
			cfg.ImportBucket = cfg.S3Bucket
		}
	})
	teacher := childCommit(t, w)
	if mode == "committed-failure" {
		t.Fatal("deliberate failure after committed login, media and source")
	}
	if mode == "same-store" && len(fixtures.buckets) != 1 {
		t.Fatal("the same configured store became two buckets")
	}
	if mode == "no-external-wake" {
		imported := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/imports", map[string]any{"requestId": uuid.NewString(), "title": "Wake"})
		source := teacher.send(http.MethodPost, "/teacher/imports/"+id(imported)+"/sources?role=exam&uploadId="+uuid.NewString()+"&expectedRevision="+jsonNumber(imported["revision"]), new(filePayload(t, "wake.docx", tinyDocx(t))))
		if source.status != http.StatusOK && source.status != http.StatusCreated {
			t.Fatalf("wake source status%d", source.status)
		}
		teacher.must(http.StatusAccepted, http.MethodPost, "/teacher/imports/"+id(imported)+"/process", map[string]any{"requestId": uuid.NewString(), "expectedRevision": source.json["import"].(map[string]any)["revision"]})
		deadline := time.Now().Add(time.Second)
		for fixtures.wakeCalls.Load() == 0 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if fixtures.wakeCalls.Load() == 0 {
			t.Fatal("the local inert wake never arrived")
		}
		if externalCalls.Load() != 0 {
			t.Fatal("the harness woke an external worker")
		}
	}
	if mode == "hold" {
		resources := []fixtureResource{{Kind: "database", Name: fixtures.database}}
		for _, bucket := range fixtures.buckets {
			resources = append(resources, fixtureResource{Kind: "bucket", Name: bucket.name, Endpoint: bucket.endpoint})
		}
		raw, err := json.Marshal(resources)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(os.Getenv("QUIZZIVY_E2E_READY"), raw, 0600); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(30 * time.Second)
		for {
			_, err := os.Stat(os.Getenv("QUIZZIVY_E2E_RELEASE"))
			if err == nil {
				break
			}
			if !errors.Is(err, os.ErrNotExist) || time.Now().After(deadline) {
				t.Fatal("child release did not arrive")
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
}

func requireFixturePresent(t *testing.T, conn *pgx.Conn, resources []fixtureResource) {
	t.Helper()
	client, err := fixtureS3(context.Background(), fixtureStorageConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, resource := range resources {
		if resource.Kind == "database" {
			var exists bool
			if err := conn.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)`, resource.Name).Scan(&exists); err != nil {
				t.Fatal(err)
			}
			if !exists {
				t.Errorf("another invocation removed live database%s", resource.Name)
			}
		} else {
			if _, err := client.HeadBucket(context.Background(), &s3.HeadBucketInput{Bucket: aws.String(resource.Name)}); err != nil {
				t.Errorf("another invocation removed live bucket%s: %v", resource.Name, err)
			}
		}
	}
}

func TestFixtureLifecycleConcurrentInvocationsPreserveASentinel(t *testing.T) {
	conn := lifecycleControl(t)
	cfg := fixtureStorageConfig()
	client, err := fixtureS3(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	dbName := "qve2e_sentinel_" + suffix
	bucketName := "qve2e-sentinel-" + suffix
	if _, err := conn.Exec(context.Background(), "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize()+" TEMPLATE template0"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { recoverFixtureDatabase(t, conn, dbName) })
	if _, err := client.CreateBucket(context.Background(), &s3.CreateBucketInput{Bucket: aws.String(bucketName)}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { recoverFixtureBucket(t, client, bucketName) })
	if _, err := client.PutObject(context.Background(), &s3.PutObjectInput{Bucket: aws.String(bucketName), Key: aws.String("sentinel"), Body: strings.NewReader("untouched sentinel")}); err != nil {
		t.Fatal(err)
	}
	sentinel := []fixtureResource{{Kind: "database", Name: dbName}, {Kind: "bucket", Name: bucketName}}
	dir := t.TempDir()
	first, firstOut := lifecycleChild(t, "hold")
	second, secondOut := lifecycleChild(t, "hold")
	first.Env = append(first.Env, "QUIZZIVY_E2E_READY="+dir+"/first", "QUIZZIVY_E2E_RELEASE="+dir+"/release-first")
	second.Env = append(second.Env, "QUIZZIVY_E2E_READY="+dir+"/second", "QUIZZIVY_E2E_RELEASE="+dir+"/release-second")
	firstWaited, secondWaited := false, false
	if err := first.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !firstWaited {
			if err := os.WriteFile(dir+"/release-first", nil, 0600); err != nil {
				t.Error(err)
			}
			if err := first.Wait(); err != nil {
				t.Errorf("release first child: %v %s", err, firstOut.String())
			}
		}
	})
	if err := second.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !secondWaited {
			if err := os.WriteFile(dir+"/release-second", nil, 0600); err != nil {
				t.Error(err)
			}
			if err := second.Wait(); err != nil {
				t.Errorf("release second child: %v %s", err, secondOut.String())
			}
		}
	})
	resources := make([][]fixtureResource, 2)
	deadline := time.Now().Add(30 * time.Second)
	for i, name := range []string{"first", "second"} {
		for {
			raw, err := os.ReadFile(dir + "/" + name)
			if err == nil {
				if err := json.Unmarshal(raw, &resources[i]); err != nil {
					t.Fatal(err)
				}
				break
			}
			if !errors.Is(err, os.ErrNotExist) || time.Now().After(deadline) {
				t.Fatal("concurrent child readiness did not arrive")
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	names := map[string]bool{}
	for _, set := range resources {
		for _, resource := range set {
			if names[resource.Name] {
				t.Errorf("concurrent invocations shared%s", resource.Name)
			}
			names[resource.Name] = true
		}
	}
	if err := os.WriteFile(dir+"/release-first", nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := first.Wait(); err != nil {
		t.Errorf("first child: %v %s", err, firstOut.String())
	}
	firstWaited = true
	requireFixtureAbsent(t, conn, resources[0])
	requireFixturePresent(t, conn, resources[1])
	requireFixturePresent(t, conn, sentinel)
	if err := os.WriteFile(dir+"/release-second", nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := second.Wait(); err != nil {
		t.Errorf("second child: %v %s", err, secondOut.String())
	}
	secondWaited = true
	requireFixtureAbsent(t, conn, resources[1])
	requireFixturePresent(t, conn, sentinel)
	object, err := client.GetObject(context.Background(), &s3.GetObjectInput{Bucket: aws.String(bucketName), Key: aws.String("sentinel")})
	if err != nil {
		t.Fatal(err)
	}
	raw, readErr := io.ReadAll(object.Body)
	closeErr := object.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		t.Fatal(err)
	}
	if string(raw) != "untouched sentinel" {
		t.Fatal("sentinel content changed")
	}
	t.Log(firstOut.String())
	t.Log(secondOut.String())
}
