//go:build integration

package db_test

import (
	"database/sql"
	"os"
	"testing"

	"github.com/pressly/goose/v3"

	"quizzivy/internal/platform/db"
)

func TestImportOverLimitRollbackFoldsOverLimitRowsIntoSkipped(t *testing.T) {
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
	if err := goose.UpTo(conn, db.MigrationsDir(t), 144); err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(`WITH student AS (
		SELECT id FROM app.roles WHERE builtin_key = 'student'
	), admin AS (
		INSERT INTO app.users (email, full_name, role_id)
		SELECT 'nhap@example.test', 'Người nhập', id FROM app.roles WHERE builtin_key = 'admin' RETURNING id
	), job AS (
		INSERT INTO app.user_import_jobs (created_by, request_id, filename, format, default_role_id, delivery, status,
		                                  total_rows, created_count, skipped_count, over_limit_count, attempts, started_at, completed_at)
		SELECT admin.id, gen_random_uuid(), 'nguoi-dung.csv', 'csv', student.id, 'sheet', 'done', 3, 1, 1, 1, 1, now(), now()
		  FROM admin, student RETURNING id
	)
	INSERT INTO app.user_import_rows (job_id, ordinal, full_name, email, role_id, outcome)
	SELECT job.id, n, 'Hàng ' || n, 'hang' || n || '@example.test', student.id,
	       (ARRAY['created', 'skipped_exists', 'skipped_limit'])[n + 1]
	  FROM job, student, generate_series(0, 2) AS n`)
	if err != nil {
		t.Fatal(err)
	}
	if err := goose.DownTo(conn, db.MigrationsDir(t), 143); err != nil {
		t.Fatalf("rolling back 00144 with an over-limit row: %v", err)
	}
	var created, skipped, limited int
	if err := conn.QueryRow(`SELECT created_count, skipped_count FROM app.user_import_jobs`).Scan(&created, &skipped); err != nil {
		t.Fatal(err)
	}
	if created != 1 || skipped != 2 {
		t.Errorf("counts after rollback = created %d skipped %d, want 1 and 2", created, skipped)
	}
	if err := conn.QueryRow(`SELECT count(*) FROM app.user_import_rows WHERE outcome = 'skipped_exists'`).Scan(&limited); err != nil || limited != 2 {
		t.Errorf("skipped rows after rollback = %d, %v; want 2", limited, err)
	}
	if err := goose.UpTo(conn, db.MigrationsDir(t), 144); err != nil {
		t.Fatalf("up again: %v", err)
	}
}
