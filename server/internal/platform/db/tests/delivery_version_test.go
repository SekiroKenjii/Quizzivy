//go:build integration

package db_test

import (
	"database/sql"
	"os"
	"testing"

	"quizzivy/internal/platform/db"

	"github.com/pressly/goose/v3"
)

func TestDeliveryMigrationPreservesHistoricalFormats(t *testing.T) {
	if os.Getenv("TEST_DESTRUCTIVE") != "1" {
		t.Skip("TEST_DESTRUCTIVE=1 required for an isolated migration database")
	}
	conn, err := sql.Open("pgx", scratchDatabase(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	goose.SetLogger(goose.NopLogger())
	dir := db.MigrationsDir(t)
	if err := goose.UpTo(conn, dir, 40); err != nil {
		t.Fatal(err)
	}
	before := schemaSnapshot(t, conn)
	if err := goose.UpTo(conn, dir, 41); err != nil {
		t.Fatal(err)
	}
	if err := goose.DownTo(conn, dir, 40); err != nil {
		t.Fatal(err)
	}
	if schemaSnapshot(t, conn) != before {
		t.Fatal("empty rollback changed prior schema")
	}
	var oldSection, groupedSection string
	withTx(t, conn, func(tx *sql.Tx, f fixture) {
		oldSection = newVersion(t, tx, f.adminID)
		groupedSection = newVersion(t, tx, f.adminID)
		newFrozenGroup(t, tx, groupedSection)
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	})
	if err := goose.UpTo(conn, dir, 41); err != nil {
		t.Fatal(err)
	}
	for section, want := range map[string]string{oldSection: "section_v1", groupedSection: "group_v1"} {
		var got string
		if err := conn.QueryRow(`SELECT v.delivery_version FROM app.test_versions v JOIN app.test_version_sections s ON s.test_version_id=v.id WHERE s.id=$1`, section).Scan(&got); err != nil || got != want {
			t.Fatalf("section %s: delivery %q, want %q: %v", section, got, want, err)
		}
	}
	before = schemaSnapshot(t, conn)
	if err := goose.DownTo(conn, dir, 40); err == nil {
		t.Fatal("rollback erased the group delivery marker")
	}
	if schemaSnapshot(t, conn) != before {
		t.Fatal("refused rollback changed schema")
	}
	version, err := goose.GetDBVersion(conn)
	if err != nil || version != 41 {
		t.Fatalf("version after refusal: %d, %v", version, err)
	}
}
