//go:build integration

package application_test

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"quizzivy/internal/platform/db"
)

func inDatabase(t *testing.T, dsn, name string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("a database URL is not a URL: %v", err)
	}
	u.Path = "/" + name
	return u.String()
}

func scratchPools(t *testing.T) (admin, app *pgxpool.Pool) {
	t.Helper()
	if os.Getenv("TEST_DESTRUCTIVE") != "1" {
		t.Skip("TEST_DESTRUCTIVE=1 not set; skipping the test that needs a database of its own")
	}
	migrateDSN := db.TestDSN(t)
	appDSN := os.Getenv("APP_DATABASE_URL")
	if appDSN == "" {
		t.Skip("APP_DATABASE_URL is not set")
	}
	name := "qv_admins_" + nonce(t)
	control, err := sql.Open("pgx", migrateDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		defer func() { _ = control.Close() }()
		var err error
		for range 20 {
			if _, err = control.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`); err == nil {
				return
			}
			time.Sleep(250 * time.Millisecond)
		}
		t.Errorf("dropping %s: %v", name, err)
	})
	if _, err := control.Exec(`CREATE DATABASE ` + name); err != nil {
		t.Fatalf("creating %s: %v", name, err)
	}

	conn, err := sql.Open("pgx", inDatabase(t, migrateDSN, name))
	if err != nil {
		t.Fatal(err)
	}
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	goose.SetLogger(goose.NopLogger())
	err = goose.Up(conn, db.MigrationsDir(t))
	_ = conn.Close()
	if err != nil {
		t.Fatalf("migrating %s: %v", name, err)
	}

	admin, err = pgxpool.New(context.Background(), inDatabase(t, migrateDSN, name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	app, err = pgxpool.New(context.Background(), inDatabase(t, appDSN, name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Close)
	var role string
	if err := app.QueryRow(context.Background(), `SELECT current_user`).Scan(&role); err != nil || role != "quizzivy_app" {
		t.Fatalf("the app pool connects as %q (%v), want quizzivy_app", role, err)
	}
	return admin, app
}
