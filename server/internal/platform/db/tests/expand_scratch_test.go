//go:build integration

package db_test

import (
	"database/sql"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"

	"quizzivy/internal/platform/db"
)

func expandScratch(t *testing.T) string {
	t.Helper()
	if os.Getenv("TEST_DESTRUCTIVE") != "1" {
		t.Skip("TEST_DESTRUCTIVE=1 not set; skipping the expand test, which creates and drops a scratch database")
	}
	admin, err := sql.Open("pgx", db.TestDSN(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = admin.Close() }()
	const name = "quizzivy_expand_r2"
	if _, err := admin.Exec(`DROP DATABASE IF EXISTS ` + name); err != nil {
		t.Fatalf("dropping a leftover %s: %v", name, err)
	}
	if _, err := admin.Exec(`CREATE DATABASE ` + name); err != nil {
		t.Fatalf("creating %s: %v", name, err)
	}
	t.Cleanup(func() {
		cleanup, err := sql.Open("pgx", db.TestDSN(t))
		if err != nil {
			t.Errorf("open for cleanup: %v", err)
			return
		}
		defer func() { _ = cleanup.Close() }()
		dropScratchDatabase(t, cleanup, name)
	})
	return swapDatabase(t, db.TestDSN(t), name)
}

func versionBefore(t *testing.T, dir, suffix string) int64 {
	t.Helper()
	migrations, err := goose.CollectMigrations(dir, 0, math.MaxInt64)
	if err != nil {
		t.Fatalf("collect migrations: %v", err)
	}
	for i, m := range migrations {
		if strings.HasSuffix(filepath.Base(m.Source), suffix) {
			if i == 0 {
				t.Fatalf("%s is the first migration", suffix)
			}
			return migrations[i-1].Version
		}
	}
	t.Fatalf("no migration ends in %s", suffix)
	return 0
}
