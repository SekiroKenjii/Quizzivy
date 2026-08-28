package db

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestDSN returns the migrate-role DSN for integration tests, or skips.
//
// Skipping rather than failing keeps `go test ./...` useful on a machine with
// no database, while CI sets the variable and therefore always runs these.
func TestDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping database integration test")
	}
	return dsn
}

// MigrationsDir resolves migrations/ relative to this source file, so tests do
// not depend on the working directory.
func MigrationsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller for migrations path")
	}
	// server/internal/db -> server/internal -> server -> repo root
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations")
}
