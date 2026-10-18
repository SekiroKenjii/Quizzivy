//go:build integration

package maintenance_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"quizzivy/internal/platform/db"
)

const maintenanceDSN = "TEST_MAINTENANCE_DATABASE_URL"

var shared struct {
	once  sync.Once
	name  string
	owner string
}

func TestMain(m *testing.M) {
	code := m.Run()
	if shared.name != "" {
		if err := dropDatabase(os.Getenv("TEST_DATABASE_URL"), shared.name); err != nil {
			fmt.Fprintln(os.Stderr, err)
			code = 1
		}
	}
	os.Exit(code)
}

func databaseURL(t *testing.T, dsn, name string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("a database URL is not a URL: %v", err)
	}
	u.Path = "/" + name
	return u.String()
}

func dropDatabase(ownerDSN, name string) error {
	var err error
	for range 3 {
		if err = dropOnce(ownerDSN, name); err == nil {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("dropping the scratch database %s: %w", name, err)
}

func dropOnce(ownerDSN, name string) error {
	admin, err := sql.Open("pgx", ownerDSN)
	if err != nil {
		return err
	}
	defer func() { _ = admin.Close() }()
	_, err = admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`)
	return err
}

func dropAfterFailure(t *testing.T, dsn, name string) {
	t.Helper()
	if err := dropDatabase(dsn, name); err != nil {
		t.Log(err)
	}
}

func createScratch(t *testing.T) (name, ownerURL string) {
	t.Helper()
	dsn := db.TestDSN(t)
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	name = "qvmaint_" + hex.EncodeToString(raw)
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = admin.Close() }()
	if _, err := admin.Exec(`CREATE DATABASE ` + name); err != nil {
		t.Fatalf("creating %s needs CREATEDB on the migrate role (scripts/provision-db.sh): %v", name, err)
	}
	ownerURL = databaseURL(t, dsn, name)
	conn, err := sql.Open("pgx", ownerURL)
	if err != nil {
		dropAfterFailure(t, dsn, name)
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.Exec(`REVOKE ALL ON SCHEMA public FROM PUBLIC; GRANT USAGE ON SCHEMA public TO quizzivy_migrate`); err != nil {
		dropAfterFailure(t, dsn, name)
		t.Fatal(err)
	}
	if err := goose.SetDialect("postgres"); err != nil {
		dropAfterFailure(t, dsn, name)
		t.Fatal(err)
	}
	goose.SetLogger(goose.NopLogger())
	if err := goose.Up(conn, db.MigrationsDir(t)); err != nil {
		dropAfterFailure(t, dsn, name)
		t.Fatalf("migrating %s: %v", name, err)
	}
	return name, ownerURL
}

func sharedScratch(t *testing.T) string {
	t.Helper()
	shared.once.Do(func() {
		shared.name, shared.owner = createScratch(t)
	})
	if shared.owner == "" {
		t.Skip("the shared scratch database could not be created")
	}
	return shared.owner
}

type isolated struct {
	name  string
	ctx   context.Context
	owner *pgxpool.Pool
	role  *pgxpool.Pool
}

func newIsolated(t *testing.T) isolated {
	t.Helper()
	dsn := os.Getenv(maintenanceDSN)
	if dsn == "" && os.Getenv("CI") != "" {
		t.Fatalf("%s is not set in CI; the tests that run as the maintenance role must not be skipped", maintenanceDSN)
	}
	if dsn == "" {
		t.Skipf("%s is not set; skipping the tests that run as the maintenance role", maintenanceDSN)
	}
	name, ownerURL := createScratch(t)
	t.Cleanup(func() {
		if err := dropDatabase(db.TestDSN(t), name); err != nil {
			t.Error(err)
		}
	})
	ctx := context.Background()
	owner, err := pgxpool.New(ctx, ownerURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(owner.Close)
	config, err := pgxpool.ParseConfig(databaseURL(t, dsn, name))
	if err != nil {
		t.Fatal(err)
	}
	config.MaxConns = 2
	role, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(role.Close)
	return isolated{name: name, ctx: ctx, owner: owner, role: role}
}
