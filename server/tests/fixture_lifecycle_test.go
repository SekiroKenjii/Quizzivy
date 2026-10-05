//go:build e2e

package e2e

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"quizzivy/internal/platform/config"
)

type fixtureResources struct {
	originalDSN string
	database    string
	control     *pgx.Conn
	buckets     []*fixtureBucket
	stores      map[string]*fixtureBucket
	wake        *httptest.Server
	wakeCalls   atomic.Int64
	inventory   *fixtureInventory
	mu          sync.Mutex
}

type fixtureBucket struct {
	endpoint string
	name     string
	client   *s3.Client
}

type fixtureResource struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Endpoint string `json:"endpoint,omitempty"`
}

var fixtures *fixtureResources

// TestMain owns one migrated database and its configured buckets until every test finishes.
func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		os.Exit(m.Run())
	}
	fixtures = &fixtureResources{originalDSN: dsn, stores: make(map[string]*fixtureBucket)}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	err := fixtures.prepare(ctx)
	cancel()
	code := 1
	if err == nil {
		code = m.Run()
	} else {
		fmt.Fprintln(os.Stderr, "E2E fixture setup:", err)
	}
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
	cleanupErr := fixtures.close(cleanupCtx)
	cleanupCancel()
	restoreErr := os.Setenv("TEST_DATABASE_URL", dsn)
	if err := errors.Join(cleanupErr, restoreErr); err != nil {
		fmt.Fprintln(os.Stderr, "E2E fixture cleanup:", err)
		code = 1
	}
	os.Exit(code)
}

func fixtureName() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "qve2e-" + hex.EncodeToString(raw[:]), nil
}

func fixtureDSN(dsn, database string) (string, error) {
	parsed, err := pgx.ParseConfig(dsn)
	if err != nil {
		return "", errors.New("invalid TEST_DATABASE_URL")
	}
	changed := dsn + " dbname=" + database + " application_name=quizzivy-e2e-fixture"
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return "", errors.New("invalid database URL")
		}
		u.Path = "/" + database
		q := u.Query()
		q.Set("application_name", "quizzivy-e2e-fixture")
		u.RawQuery = q.Encode()
		changed = u.String()
	}
	target, err := pgx.ParseConfig(changed)
	if err != nil || target.Database != database || target.User != parsed.User {
		return "", errors.New("could not route the fixture database")
	}
	return changed, nil
}

func (f *fixtureResources) prepare(ctx context.Context) error {
	cfg, err := pgx.ParseConfig(f.originalDSN)
	if err != nil {
		return errors.New("invalid TEST_DATABASE_URL")
	}
	cfg.RuntimeParams["application_name"] = "quizzivy-e2e-control"
	f.control, err = pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connect fixture control: %w", err)
	}
	var capable bool
	var version int
	if err := f.control.QueryRow(ctx, `SELECT rolcreatedb OR rolsuper,current_setting('server_version_num')::integer FROM pg_roles WHERE rolname=current_user`).Scan(&capable, &version); err != nil {
		return err
	}
	if !capable || version < 180000 {
		return errors.New("E2E fixtures require PostgreSQL 18 and a test role with CREATEDB")
	}
	if path := os.Getenv("QUIZZIVY_E2E_INVENTORY"); path != "" {
		before, err := readFixtureInventory(ctx, f.control)
		if err != nil {
			return err
		}
		f.inventory = &before
		if err := writeFixtureInventory(path+".before.json", before); err != nil {
			return err
		}
	}
	name, err := fixtureName()
	if err != nil {
		return err
	}
	name = strings.ReplaceAll(name, "-", "_")
	if _, err := f.control.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()+" TEMPLATE template0"); err != nil {
		return fmt.Errorf("create fixture database: %w", err)
	}
	f.database = name
	announceFixture("E2E_RESOURCE", fixtureResource{Kind: "database", Name: name})
	if os.Getenv("QUIZZIVY_E2E_PROBE") == "after-database" {
		return errors.New("injected setup failure after database creation")
	}
	target, err := fixtureDSN(f.originalDSN, name)
	if err != nil {
		return err
	}
	if err := f.migrate(ctx, target); err != nil {
		return err
	}
	return os.Setenv("TEST_DATABASE_URL", target)
}

func (f *fixtureResources) migrate(ctx context.Context, dsn string) error {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return err
	}
	conn := stdlib.OpenDB(*cfg)
	provider, err := goose.NewProvider(goose.DialectPostgres, conn, os.DirFS("../../migrations"))
	if err == nil {
		_, err = provider.Up(ctx)
	}
	if err == nil {
		_, err = conn.ExecContext(ctx, `REVOKE ALL ON SCHEMA public FROM PUBLIC; GRANT USAGE ON SCHEMA public TO quizzivy_migrate,quizzivy_app`)
	}
	return errors.Join(err, conn.Close())
}

func fixtureS3(ctx context.Context, cfg config.Config) (*s3.Client, error) {
	loaded, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.S3Region), awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.S3AccessKeyID, cfg.S3SecretAccessKey, "")), awsconfig.WithRequestChecksumCalculation(aws.RequestChecksumCalculationWhenRequired), awsconfig.WithResponseChecksumValidation(aws.ResponseChecksumValidationWhenRequired))
	if err != nil {
		return nil, err
	}
	return s3.NewFromConfig(loaded, func(o *s3.Options) {
		if cfg.S3Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.S3Endpoint)
		}
		o.UsePathStyle = cfg.S3ForcePathStyle
	}), nil
}

func announceFixture(label string, resource fixtureResource) {
	raw, err := json.Marshal(resource)
	if err != nil {
		fmt.Fprintln(os.Stderr, "E2E resource logging:", err)
		return
	}
	fmt.Fprintln(os.Stderr, label, string(raw))
}

func safeFixtureEndpoint(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "configured-endpoint"
	}
	return (&url.URL{Scheme: u.Scheme, Host: u.Host, Path: u.Path}).String()
}

func fixtureNotFound(err error) bool {
	var response interface{ HTTPStatusCode() int }
	return errors.As(err, &response) && response.HTTPStatusCode() == http.StatusNotFound
}

func (f *fixtureResources) bucket(ctx context.Context, cfg config.Config, original string) (string, error) {
	key := cfg.S3Endpoint + "\x00" + original
	if bucket := f.stores[key]; bucket != nil {
		return bucket.name, nil
	}
	client, err := fixtureS3(ctx, cfg)
	if err != nil {
		return "", err
	}
	name, err := fixtureName()
	if err != nil {
		return "", err
	}
	if _, err := client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(name)}); !fixtureNotFound(err) {
		if err == nil {
			return "", fmt.Errorf("fixture bucket name already exists: %s", name)
		}
		return "", fmt.Errorf("confirm absent fixture bucket %s: %w", name, err)
	}
	input := &s3.CreateBucketInput{Bucket: aws.String(name)}
	if cfg.S3Region != "" && cfg.S3Region != "us-east-1" {
		input.CreateBucketConfiguration = &types.CreateBucketConfiguration{LocationConstraint: types.BucketLocationConstraint(cfg.S3Region)}
	}
	if _, err := client.CreateBucket(ctx, input); err != nil {
		return "", fmt.Errorf("create fixture bucket %s: %w", name, err)
	}
	bucket := &fixtureBucket{endpoint: safeFixtureEndpoint(cfg.S3Endpoint), name: name, client: client}
	f.buckets = append(f.buckets, bucket)
	f.stores[key] = bucket
	announceFixture("E2E_RESOURCE", fixtureResource{Kind: "bucket", Name: name, Endpoint: bucket.endpoint})
	if os.Getenv("QUIZZIVY_E2E_PROBE") == "after-bucket" {
		return "", errors.New("injected setup failure after bucket creation")
	}
	return name, nil
}

func (f *fixtureResources) route(cfg *config.Config) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if cfg.MediaEnabled() {
		name, err := f.bucket(ctx, *cfg, cfg.S3Bucket)
		if err != nil {
			return err
		}
		cfg.S3Bucket = name
	}
	if cfg.ImportBucket != "" {
		name, err := f.bucket(ctx, *cfg, cfg.ImportBucket)
		if err != nil {
			return err
		}
		cfg.ImportBucket = name
	}
	if cfg.ImportProcessing && cfg.ImportBucket != "" {
		if f.wake == nil {
			f.wake = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				f.wakeCalls.Add(1)
				w.WriteHeader(http.StatusNoContent)
			}))
		}
		cfg.ImportWorkerWakeURL = f.wake.URL
	}
	return nil
}

func (b *fixtureBucket) removeBatch(ctx context.Context, objects []types.ObjectIdentifier) error {
	if len(objects) == 0 {
		return nil
	}
	out, err := b.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{Bucket: aws.String(b.name), Delete: &types.Delete{Objects: objects, Quiet: aws.Bool(true)}})
	if err != nil {
		return err
	}
	var failures []error
	for _, failure := range out.Errors {
		failures = append(failures, fmt.Errorf("delete fixture object %s: %s", aws.ToString(failure.Key), aws.ToString(failure.Code)))
	}
	return errors.Join(failures...)
}

func (b *fixtureBucket) emptyVersions(ctx context.Context) error {
	status, err := b.client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: aws.String(b.name)})
	if err != nil {
		return err
	}
	if status.Status == "" {
		return nil
	}
	var objects []types.ObjectIdentifier
	pages := s3.NewListObjectVersionsPaginator(b.client, &s3.ListObjectVersionsInput{Bucket: aws.String(b.name), MaxKeys: aws.Int32(1000)})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return err
		}
		for _, v := range page.Versions {
			objects = append(objects, types.ObjectIdentifier{Key: v.Key, VersionId: v.VersionId})
		}
		for _, v := range page.DeleteMarkers {
			objects = append(objects, types.ObjectIdentifier{Key: v.Key, VersionId: v.VersionId})
		}
	}
	for len(objects) > 0 {
		count := min(len(objects), 1000)
		if err := b.removeBatch(ctx, objects[:count]); err != nil {
			return err
		}
		objects = objects[count:]
	}
	return nil
}

func (b *fixtureBucket) empty(ctx context.Context) error {
	versionErr := b.emptyVersions(ctx)
	pages := s3.NewListObjectsV2Paginator(b.client, &s3.ListObjectsV2Input{Bucket: aws.String(b.name), MaxKeys: aws.Int32(1000)})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return errors.Join(versionErr, err)
		}
		objects := make([]types.ObjectIdentifier, 0, len(page.Contents))
		for _, v := range page.Contents {
			objects = append(objects, types.ObjectIdentifier{Key: v.Key})
		}
		if err := b.removeBatch(ctx, objects); err != nil {
			return errors.Join(versionErr, err)
		}
	}
	return versionErr
}

func (f *fixtureResources) close(ctx context.Context) error {
	ctx = context.WithoutCancel(ctx)
	var failures []error
	if f.wake != nil {
		f.wake.Close()
	}
	for i := len(f.buckets) - 1; i >= 0; i-- {
		bucket := f.buckets[i]
		bucketCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		emptyErr := bucket.empty(bucketCtx)
		cancel()
		deleteCtx, deleteCancel := context.WithTimeout(ctx, 15*time.Second)
		_, deleteErr := bucket.client.DeleteBucket(deleteCtx, &s3.DeleteBucketInput{Bucket: aws.String(bucket.name)})
		deleteCancel()
		failures = append(failures, emptyErr, deleteErr)
		if emptyErr == nil && deleteErr == nil {
			announceFixture("E2E_REMOVED", fixtureResource{Kind: "bucket", Name: bucket.name, Endpoint: bucket.endpoint})
		}
	}
	if f.database != "" {
		dropCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		_, err := f.control.Exec(dropCtx, "DROP DATABASE "+pgx.Identifier{f.database}.Sanitize())
		cancel()
		failures = append(failures, err)
		if err == nil {
			announceFixture("E2E_REMOVED", fixtureResource{Kind: "database", Name: f.database})
		}
	}
	if f.inventory != nil {
		inventoryCtx, cancel := context.WithTimeout(ctx, time.Minute)
		after, err := readFixtureInventory(inventoryCtx, f.control)
		cancel()
		failures = append(failures, err)
		if err == nil {
			failures = append(failures, writeFixtureInventory(os.Getenv("QUIZZIVY_E2E_INVENTORY")+".after.json", after))
			if !reflect.DeepEqual(*f.inventory, after) {
				failures = append(failures, errors.New("the parent row/ID, role or object/version inventory changed"))
			}
		}
	}
	if f.control != nil {
		controlCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		failures = append(failures, f.control.Close(controlCtx))
		cancel()
	}
	if os.Getenv("QUIZZIVY_E2E_PROBE") == "cleanup-error" {
		failures = append(failures, errors.New("injected cleanup failure"))
	}
	return errors.Join(failures...)
}
