//go:build integration

package db_test

import (
	"database/sql"
	"os"
	"testing"

	"github.com/pressly/goose/v3"

	"quizzivy/internal/platform/db"
)

// The reference tables of DG-109 (R6-12), by the version each side leaves.
const (
	beforeMediaReferences int64 = 153
	mediaReferences       int64 = 154
)

// TestTheMediaReferenceRollbackRefusesWhileARowExists is QC-R6M-1 for 00154:
// without its rows an image a published paper draws could be deleted, so the
// Down refuses while either table holds one, and once both are empty it puts
// the schema back exactly as it found it.
func TestTheMediaReferenceRollbackRefusesWhileARowExists(t *testing.T) {
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
	if err := goose.UpTo(conn, dir, beforeMediaReferences); err != nil {
		t.Fatal(err)
	}
	before := schemaSnapshot(t, conn)
	if err := goose.UpTo(conn, dir, mediaReferences); err != nil {
		t.Fatal(err)
	}

	var question, frozen string
	withTx(t, conn, func(tx *sql.Tx, f fixture) {
		var asset string
		if err := tx.QueryRow(`INSERT INTO app.media_assets (kind, storage_key, mime_type, bytes, original_filename, checksum_sha256, uploaded_by, owner_id)
			VALUES ('image', 'rollback/hinh.png', 'image/png', 1, 'hinh.png', sha256('hinh'::bytea), $1, $1) RETURNING id`, f.adminID).Scan(&asset); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRow(`INSERT INTO app.questions (type, prompt, points, created_by, owner_id)
			VALUES ('short_answer', 'Nhìn hình', 1.0, $1, $1) RETURNING id`, f.adminID).Scan(&question); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRow(`INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, points)
			VALUES ($1, 0, 'short_answer', 'Nhìn hình', 1.0) RETURNING id`, newVersion(t, tx, f.adminID)).Scan(&frozen); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(`INSERT INTO app.question_media_references (question_id, field, media_asset_id, media_asset_kind)
			VALUES ($1, 'prompt', $2, 'image')`, question, asset); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(`INSERT INTO app.test_version_question_media_references (test_version_question_id, field, media_asset_id, media_asset_kind)
			VALUES ($1, 'explanation', $2, 'image')`, frozen, asset); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	})

	for _, step := range []struct {
		name, clear, id string
	}{
		{"a bank question's reference", `DELETE FROM app.question_media_references WHERE question_id = $1`, question},
		{"a frozen question's reference", `DELETE FROM app.test_version_question_media_references WHERE test_version_question_id = $1`, frozen},
	} {
		if err := goose.DownTo(conn, dir, beforeMediaReferences); err == nil {
			t.Fatalf("the rollback ran while %s exists", step.name)
		}
		if version, err := goose.GetDBVersion(conn); err != nil || version != mediaReferences {
			t.Fatalf("version after refusing %s: %d, %v; want %d", step.name, version, err, mediaReferences)
		}
		if _, err := conn.Exec(step.clear, step.id); err != nil {
			t.Fatal(err)
		}
	}
	if err := goose.DownTo(conn, dir, beforeMediaReferences); err != nil {
		t.Fatalf("the rollback refused with no reference left: %v", err)
	}
	if after := schemaSnapshot(t, conn); after != before {
		t.Errorf("the rollback left the schema changed\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}
}
