//go:build integration

package db_test

import (
	"database/sql"
	"os"
	"testing"

	"github.com/pressly/goose/v3"

	"quizzivy/internal/platform/db"
)

func TestGapRollbackPreservesWrittenBindings(t *testing.T) {
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
	if err := goose.UpTo(conn, db.MigrationsDir(t), 34); err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(`WITH author AS (
		INSERT INTO app.users (email, full_name, role)
		VALUES ('rollback@example.test', 'Rollback', 'admin') RETURNING id
	), question AS (
		INSERT INTO app.questions (type, prompt, prompt_content, points, created_by)
		SELECT 'fill_blank', '[1]', '{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"gap","id":"gap-1","label":"1"}]}]}'::jsonb, 1, id
		FROM author RETURNING id
	)
	INSERT INTO app.question_blanks (question_id, ordinal, gap_id, case_sensitive)
	SELECT id, 1, 'gap-1', false FROM question`)
	if err != nil {
		t.Fatal(err)
	}
	before := schemaSnapshot(t, conn)
	if err := goose.DownTo(conn, db.MigrationsDir(t), 33); err == nil {
		t.Fatal("rollback discarded rich fill-blank content")
	}
	if after := schemaSnapshot(t, conn); after != before {
		t.Fatal("refused rollback changed the schema")
	}
	var gap string
	if err := conn.QueryRow(`SELECT gap_id FROM app.question_blanks`).Scan(&gap); err != nil || gap != "gap-1" {
		t.Fatalf("binding after rollback = %q, %v", gap, err)
	}
	version, err := goose.GetDBVersion(conn)
	if err != nil || version != 34 {
		t.Fatalf("version after rollback = %d, %v", version, err)
	}
}
