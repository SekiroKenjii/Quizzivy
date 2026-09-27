//go:build integration

package db_test

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
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
			return
		}
		defer func() { _ = cleanup.Close() }()
		_, _ = cleanup.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`)
	})
	return swapDatabase(t, db.TestDSN(t), name)
}

func openAs(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := conn.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	return conn
}

func appRoleDSN(t *testing.T, dsn string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse DSN: %v", err)
	}
	password := os.Getenv("QUIZZIVY_APP_PASSWORD")
	if password == "" {
		password = "app"
	}
	u.User = url.UserPassword(appRole, password)
	return u.String()
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

func refusedWith(t *testing.T, err error, constraint string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("want a refusal by %s, got %v", constraint, err)
	}
	if pgErr.Code != "23514" || pgErr.ConstraintName != constraint {
		t.Fatalf("want 23514 from %s, got %s from %q: %s", constraint, pgErr.Code, pgErr.ConstraintName, pgErr.Message)
	}
}

type legacyUser struct {
	id, role  string
	updatedAt time.Time
}

func TestTheR2ExpandKeepsTheOldBinaryWorking(t *testing.T) {
	dsn := expandScratch(t)
	migrate := openAs(t, dsn)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	goose.SetLogger(goose.NopLogger())
	dir := db.MigrationsDir(t)

	if err := goose.UpTo(migrate, dir, versionBefore(t, dir, "_create_roles_and_permissions.sql")); err != nil {
		t.Fatalf("up to v0.7.0: %v", err)
	}
	legacy := map[string]*legacyUser{}
	for _, u := range []struct{ email, role string }{
		{"owner@example.com", "admin"},
		{"second-admin@example.com", "admin"},
		{"lan@example.com", "student"},
		{"minh@example.com", "student"},
		{"an@example.com", "student"},
	} {
		lu := &legacyUser{role: u.role}
		if err := migrate.QueryRow(`
			INSERT INTO app.users (email, full_name, role, created_at, updated_at)
			VALUES ($1, $1, $2::app.user_role, now() - interval '30 days', now() - interval '30 days')
			RETURNING id::text, updated_at`, u.email, u.role).Scan(&lu.id, &lu.updatedAt); err != nil {
			t.Fatalf("legacy insert %s: %v", u.email, err)
		}
		legacy[u.email] = lu
	}
	if err := goose.Up(migrate, dir); err != nil {
		t.Fatalf("up: %v", err)
	}

	for email, lu := range legacy {
		var builtin string
		var updatedAt time.Time
		if err := migrate.QueryRow(`
			SELECT r.builtin_key, u.updated_at
			  FROM app.users u JOIN app.roles r ON r.id = u.role_id
			 WHERE u.id = $1`, lu.id).Scan(&builtin, &updatedAt); err != nil {
			t.Fatalf("%s after the backfill: %v", email, err)
		}
		if builtin != lu.role {
			t.Errorf("%s: role_id is the %s role, want %s", email, builtin, lu.role)
		}
		if !updatedAt.Equal(lu.updatedAt) {
			t.Errorf("%s: updated_at moved from %v to %v", email, lu.updatedAt, updatedAt)
		}
	}

	app := openAs(t, appRoleDSN(t, dsn))

	t.Run("an insert with only role gets role_id", func(t *testing.T) {
		var builtin string
		if err := app.QueryRow(`
			WITH ins AS (
			  INSERT INTO app.users (email, full_name, role) VALUES ('old-binary@example.com', 'Old', 'student')
			  RETURNING role_id)
			SELECT r.builtin_key FROM ins JOIN app.roles r ON r.id = ins.role_id`).Scan(&builtin); err != nil {
			t.Fatal(err)
		}
		if builtin != "student" {
			t.Errorf("role_id is the %s role, want student", builtin)
		}
	})

	t.Run("an insert with role_id derives role", func(t *testing.T) {
		for builtin, want := range map[string]string{"teacher": "admin", "student": "student", "assistant": "admin"} {
			var role string
			if err := app.QueryRow(`
				INSERT INTO app.users (email, full_name, role_id)
				VALUES ($1, 'New', (SELECT id FROM app.roles WHERE builtin_key = $2))
				RETURNING role::text`, "new-"+builtin+"@example.com", builtin).Scan(&role); err != nil {
				t.Fatalf("%s: %v", builtin, err)
			}
			if role != want {
				t.Errorf("role_id %s gave role %q, want %q", builtin, role, want)
			}
		}
	})

	t.Run("an update of role alone re-derives role_id", func(t *testing.T) {
		tx, err := app.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		var builtin string
		if err := tx.QueryRow(`
			WITH up AS (UPDATE app.users SET role = 'admin' WHERE id = $1 RETURNING role_id)
			SELECT r.builtin_key FROM up JOIN app.roles r ON r.id = up.role_id`, legacy["lan@example.com"].id).Scan(&builtin); err != nil {
			t.Fatal(err)
		}
		if builtin != "admin" {
			t.Errorf("role_id is the %s role, want admin", builtin)
		}
	})

	if _, err := app.Exec(`UPDATE app.users SET disabled_at = now() WHERE id = $1`, legacy["second-admin@example.com"].id); err != nil {
		t.Fatalf("disabling one of two admins: %v", err)
	}
	owner := legacy["owner@example.com"].id

	t.Run("the last active admin cannot leave", func(t *testing.T) {
		for name, statement := range map[string]string{
			"demote by role_id":  `UPDATE app.users SET role_id = (SELECT id FROM app.roles WHERE builtin_key = 'teacher') WHERE id = $1`,
			"demote by role":     `UPDATE app.users SET role = 'student' WHERE id = $1`,
			"disable":            `UPDATE app.users SET disabled_at = now() WHERE id = $1`,
			"delete":             `DELETE FROM app.users WHERE id = $1`,
			"demote and disable": `UPDATE app.users SET role_id = (SELECT id FROM app.roles WHERE builtin_key = 'student'), disabled_at = now() WHERE id = $1`,
		} {
			t.Run(name, func(t *testing.T) {
				tx, err := app.Begin()
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = tx.Rollback() }()
				_, err = tx.Exec(statement, owner)
				refusedWith(t, err, "users_last_admin")
			})
		}
	})

	t.Run("a student change takes no lock on the Admin role", func(t *testing.T) {
		tx, err := app.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		if _, err := tx.Exec(`UPDATE app.users SET disabled_at = now() WHERE id = $1`, legacy["minh@example.com"].id); err != nil {
			t.Fatal(err)
		}
		var locked int
		if err := migrate.QueryRow(`
			SELECT count(*)
			  FROM pg_locks l
			  JOIN pg_class c ON c.oid = l.relation
			 WHERE c.relname = 'roles' AND l.mode = 'RowShareLock' AND l.granted AND l.pid <> pg_backend_pid()`).Scan(&locked); err != nil {
			t.Fatal(err)
		}
		if locked != 0 {
			t.Errorf("disabling a student took %d row lock(s) on app.roles", locked)
		}
	})

	t.Run("of two transactions disabling the only two admins exactly one commits", func(t *testing.T) {
		if _, err := migrate.Exec(`UPDATE app.users SET disabled_at = NULL WHERE id = $1`, legacy["second-admin@example.com"].id); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		first, err := app.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = first.Rollback() }()
		second, err := app.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = second.Rollback() }()
		var secondPID int
		if err := second.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&secondPID); err != nil {
			t.Fatal(err)
		}
		if _, err := first.ExecContext(ctx, `UPDATE app.users SET disabled_at = now() WHERE id = $1`, owner); err != nil {
			t.Fatalf("first disable: %v", err)
		}
		done := make(chan error, 1)
		go func() {
			_, err := second.ExecContext(ctx, `UPDATE app.users SET disabled_at = now() WHERE id = $1`, legacy["second-admin@example.com"].id)
			done <- err
		}()
		for {
			var waiting bool
			if err := app.QueryRowContext(ctx, `SELECT coalesce(wait_event_type = 'Lock', false) FROM pg_stat_activity WHERE pid = $1`, secondPID).Scan(&waiting); err != nil {
				t.Fatal(err)
			}
			if waiting {
				break
			}
			select {
			case <-ctx.Done():
				t.Fatal("the second disable never waited on the first")
			case <-time.After(20 * time.Millisecond):
			}
		}
		if err := first.Commit(); err != nil {
			t.Fatalf("first commit: %v", err)
		}
		refusedWith(t, <-done, "users_last_admin")
		var active int
		if err := migrate.QueryRow(`
			SELECT count(*) FROM app.users u JOIN app.roles r ON r.id = u.role_id
			 WHERE r.builtin_key = 'admin' AND u.disabled_at IS NULL`).Scan(&active); err != nil {
			t.Fatal(err)
		}
		if active != 1 {
			t.Errorf("%d active admins remain, want 1", active)
		}
	})

	t.Run("a foreign-key check on the Admin role does not hold up the guard", func(t *testing.T) {
		if _, err := migrate.Exec(`UPDATE app.users SET disabled_at = NULL WHERE id = ANY($1::uuid[])`,
			[]string{owner, legacy["second-admin@example.com"].id}); err != nil {
			t.Fatal(err)
		}
		promoting, err := app.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = promoting.Rollback() }()
		if _, err := promoting.Exec(`UPDATE app.users SET role_id = (SELECT id FROM app.roles WHERE builtin_key = 'admin') WHERE id = $1`, legacy["an@example.com"].id); err != nil {
			t.Fatalf("promoting a student: %v", err)
		}
		disabling, err := app.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = disabling.Rollback() }()
		if _, err := disabling.Exec(`SET LOCAL lock_timeout = '2s'`); err != nil {
			t.Fatal(err)
		}
		if _, err := disabling.Exec(`UPDATE app.users SET disabled_at = now() WHERE id = $1`, legacy["second-admin@example.com"].id); err != nil {
			t.Errorf("disabling an Admin while another transaction's foreign-key check holds the Admin role: %v", err)
		}
	})

	t.Run("the guard runs as its owner with a pinned search path", func(t *testing.T) {
		var definer bool
		var owner, config string
		if err := migrate.QueryRow(`
			SELECT p.prosecdef, pg_get_userbyid(p.proowner), coalesce(array_to_string(p.proconfig, ';'), '')
			  FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
			 WHERE n.nspname = 'app' AND p.proname = 'users_last_admin'`).Scan(&definer, &owner, &config); err != nil {
			t.Fatal(err)
		}
		if !definer || owner != "quizzivy_migrate" || config != "search_path=pg_catalog, app, pg_temp" {
			t.Errorf("users_last_admin: prosecdef %v, owner %s, proconfig %q", definer, owner, config)
		}
		var executable bool
		if err := migrate.QueryRow(`SELECT has_function_privilege($1, 'app.users_last_admin()', 'EXECUTE')`, appRole).Scan(&executable); err != nil {
			t.Fatal(err)
		}
		if executable {
			t.Error("quizzivy_app can EXECUTE users_last_admin and attach it to a table of its own")
		}
		if hasTablePrivilege(t, migrate, appRole, "app.roles", "UPDATE") {
			t.Error("quizzivy_app can UPDATE app.roles; the guard would not need to be a definer")
		}
	})
}

type legacyRow struct {
	table, id, author string
	updatedAt         *time.Time
}

func TestTheOwnershipExpandKeepsTheOldBinaryWorking(t *testing.T) {
	dsn := expandScratch(t)
	migrate := openAs(t, dsn)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	goose.SetLogger(goose.NopLogger())
	dir := db.MigrationsDir(t)
	if err := goose.UpTo(migrate, dir, versionBefore(t, dir, "_create_roles_and_permissions.sql")); err != nil {
		t.Fatalf("up to v0.7.0: %v", err)
	}
	insertID := func(query string, args ...any) string {
		t.Helper()
		var id string
		if err := migrate.QueryRow(query, args...).Scan(&id); err != nil {
			t.Fatalf("legacy insert %q: %v", query, err)
		}
		return id
	}
	insertID(`INSERT INTO app.users (email, full_name, role, created_at, disabled_at) VALUES ('gone@example.com', 'Gone', 'admin', now() - interval '60 days', now() - interval '1 day') RETURNING id::text`)
	first := insertID(`INSERT INTO app.users (email, full_name, role, created_at) VALUES ('first@example.com', 'First', 'admin', now() - interval '50 days') RETURNING id::text`)
	second := insertID(`INSERT INTO app.users (email, full_name, role, created_at) VALUES ('second@example.com', 'Second', 'admin', now() - interval '40 days') RETURNING id::text`)
	const old = `now() - interval '30 days'`
	firstTest := insertID(`INSERT INTO app.tests (title, created_by, created_at, updated_at) VALUES ('First test', $1, `+old+`, `+old+`) RETURNING id::text`, first)
	secondTest := insertID(`INSERT INTO app.tests (title, created_by, created_at, updated_at) VALUES ('Second test', $1, `+old+`, `+old+`) RETURNING id::text`, second)
	section := insertID(`INSERT INTO app.test_sections (test_id, ordinal, title) VALUES ($1, 0, 'Part 1') RETURNING id::text`, secondTest)
	rows := []legacyRow{
		{table: "tests", id: firstTest, author: first},
		{table: "tests", id: secondTest, author: second},
		{table: "questions", id: insertID(`INSERT INTO app.questions (type, prompt, points, created_by, created_at, updated_at) VALUES ('short_answer', 'First question', 1, $1, `+old+`, `+old+`) RETURNING id::text`, first), author: first},
		{table: "questions", id: insertID(`INSERT INTO app.questions (type, prompt, points, created_by, created_at, updated_at) VALUES ('short_answer', 'Second question', 1, $1, `+old+`, `+old+`) RETURNING id::text`, second), author: second},
		{table: "question_groups", id: insertID(`INSERT INTO app.question_groups (title, created_by, created_at, updated_at) VALUES ('Bank group', $1, `+old+`, `+old+`) RETURNING id::text`, first), author: first},
		{table: "question_groups", id: insertID(`INSERT INTO app.question_groups (owner_section_id, title, created_by, created_at, updated_at) VALUES ($1, 'Section group', $2, `+old+`, `+old+`) RETURNING id::text`, section, second), author: second},
		{table: "media_assets", id: insertID(`INSERT INTO app.media_assets (kind, storage_key, mime_type, bytes, original_filename, checksum_sha256, uploaded_by) VALUES ('image', 'media/legacy.png', 'image/png', 10, 'legacy.png', sha256('legacy'::bytea), $1) RETURNING id::text`, second), author: second},
	}
	legacyClass := insertID(`INSERT INTO app.classes (name, created_at, updated_at) VALUES ('Legacy class', ` + old + `, ` + old + `) RETURNING id::text`)
	var classUpdatedAt time.Time
	if err := migrate.QueryRow(`SELECT updated_at FROM app.classes WHERE id = $1`, legacyClass).Scan(&classUpdatedAt); err != nil {
		t.Fatal(err)
	}
	for i, row := range rows {
		if row.table == "media_assets" {
			continue
		}
		var updatedAt time.Time
		if err := migrate.QueryRow(`SELECT updated_at FROM app.`+row.table+` WHERE id = $1`, row.id).Scan(&updatedAt); err != nil {
			t.Fatal(err)
		}
		rows[i].updatedAt = &updatedAt
	}

	if err := goose.Up(migrate, dir); err != nil {
		t.Fatalf("up: %v", err)
	}

	for _, row := range rows {
		var owner string
		var updatedAt *time.Time
		query := `SELECT owner_id::text, NULL::timestamptz FROM app.` + row.table + ` WHERE id = $1`
		if row.updatedAt != nil {
			query = `SELECT owner_id::text, updated_at FROM app.` + row.table + ` WHERE id = $1`
		}
		if err := migrate.QueryRow(query, row.id).Scan(&owner, &updatedAt); err != nil {
			t.Fatalf("%s %s after the backfill: %v", row.table, row.id, err)
		}
		if owner != row.author {
			t.Errorf("%s %s: owner_id %s, want its author %s", row.table, row.id, owner, row.author)
		}
		if row.updatedAt != nil && !updatedAt.Equal(*row.updatedAt) {
			t.Errorf("%s %s: updated_at moved from %v to %v", row.table, row.id, *row.updatedAt, *updatedAt)
		}
	}

	var classTeacher string
	var classUpdated time.Time
	if err := migrate.QueryRow(`SELECT teacher_id::text, updated_at FROM app.classes WHERE id = $1`, legacyClass).Scan(&classTeacher, &classUpdated); err != nil {
		t.Fatal(err)
	}
	if classTeacher != first {
		t.Errorf("the legacy class's teacher is %s, want the oldest active Admin %s", classTeacher, first)
	}
	if !classUpdated.Equal(classUpdatedAt) {
		t.Errorf("the legacy class's updated_at moved from %v to %v", classUpdatedAt, classUpdated)
	}

	app := openAs(t, appRoleDSN(t, dsn))

	t.Run("the old binary's class goes to the oldest active Admin", func(t *testing.T) {
		tx, err := app.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		var teacher string
		if err := tx.QueryRow(`INSERT INTO app.classes (name) VALUES ('Old binary') RETURNING teacher_id::text`).Scan(&teacher); err != nil {
			t.Fatal(err)
		}
		if teacher != first {
			t.Errorf("teacher_id %s, want the oldest active Admin %s", teacher, first)
		}
		var named string
		if err := tx.QueryRow(`INSERT INTO app.classes (name, teacher_id) VALUES ('New binary', $1) RETURNING teacher_id::text`, second).Scan(&named); err != nil {
			t.Fatal(err)
		}
		if named != second {
			t.Errorf("teacher_id %s, want the named teacher %s", named, second)
		}
	})

	t.Run("the old binary's inserts are given their author as owner", func(t *testing.T) {
		tx, err := app.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		for table, insert := range map[string]string{
			"tests":           `INSERT INTO app.tests (title, created_by) VALUES ('Old binary', $1) RETURNING owner_id::text`,
			"questions":       `INSERT INTO app.questions (type, prompt, points, created_by) VALUES ('short_answer', 'Old binary', 1, $1) RETURNING owner_id::text`,
			"question_groups": `INSERT INTO app.question_groups (title, created_by) VALUES ('Old binary', $1) RETURNING owner_id::text`,
			"media_assets":    `INSERT INTO app.media_assets (kind, storage_key, mime_type, bytes, original_filename, checksum_sha256, uploaded_by) VALUES ('image', 'media/old-binary.png', 'image/png', 10, 'old.png', sha256('old'::bytea), $1) RETURNING owner_id::text`,
		} {
			var owner string
			if err := tx.QueryRow(insert, second).Scan(&owner); err != nil {
				t.Fatalf("%s: %v", table, err)
			}
			if owner != second {
				t.Errorf("%s: owner_id %s, want the author %s", table, owner, second)
			}
		}
	})

	t.Run("an explicit owner is kept", func(t *testing.T) {
		tx, err := app.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		var owner string
		if err := tx.QueryRow(`INSERT INTO app.tests (title, created_by, owner_id) VALUES ('Copy', $1, $2) RETURNING owner_id::text`, first, second).Scan(&owner); err != nil {
			t.Fatal(err)
		}
		if owner != second {
			t.Errorf("owner_id %s, want the named owner %s", owner, second)
		}
	})

	t.Run("an account's creator is optional and survives its creator", func(t *testing.T) {
		tx, err := app.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		var selfJoined, created string
		var creator *string
		if err := tx.QueryRow(`INSERT INTO app.users (email, full_name, role) VALUES ('self@example.com', 'Self', 'student') RETURNING id::text, created_by::text`).Scan(&selfJoined, &creator); err != nil {
			t.Fatal(err)
		}
		if creator != nil {
			t.Errorf("a self-joined account has creator %s, want none", *creator)
		}
		var staff string
		if err := migrate.QueryRow(`INSERT INTO app.users (email, full_name, role) VALUES ('staff@example.com', 'Staff', 'admin') RETURNING id::text`).Scan(&staff); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := migrate.Exec(`DELETE FROM app.users WHERE id = $1`, staff); err != nil {
				t.Errorf("cleanup staff: %v", err)
			}
		})
		if err := tx.QueryRow(`INSERT INTO app.users (email, full_name, role, created_by) VALUES ('made@example.com', 'Made', 'student', $1) RETURNING id::text`, staff).Scan(&created); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(`DELETE FROM app.users WHERE id = $1`, staff); err != nil {
			t.Fatalf("deleting the creator: %v", err)
		}
		if err := tx.QueryRow(`SELECT created_by::text FROM app.users WHERE id = $1`, created).Scan(&creator); err != nil {
			t.Fatal(err)
		}
		if creator != nil {
			t.Errorf("the account still names its deleted creator %s", *creator)
		}
	})

	t.Run("each owner column is NOT VALID until R3 validates it", func(t *testing.T) {
		for table, constraint := range map[string]string{
			"tests":           "tests_owner_id_not_null",
			"questions":       "questions_owner_id_not_null",
			"question_groups": "question_groups_owner_id_not_null",
			"media_assets":    "media_assets_owner_id_not_null",
			"classes":         "classes_teacher_id_not_null",
		} {
			var validated bool
			if err := migrate.QueryRow(`
				SELECT c.convalidated FROM pg_constraint c
				 WHERE c.conname = $1 AND c.conrelid = ('app.' || $2)::regclass`, constraint, table).Scan(&validated); err != nil {
				t.Fatalf("%s: %v", constraint, err)
			}
			if validated {
				t.Errorf("%s is validated; R3 owns that step", constraint)
			}
		}
	})
}

func TestAddingTheClassTeacherWaitsForTheOldStudentCreateInsteadOfDeadlocking(t *testing.T) {
	dsn := expandScratch(t)
	migrate := openAs(t, dsn)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	goose.SetLogger(goose.NopLogger())
	dir := db.MigrationsDir(t)
	if err := goose.UpTo(migrate, dir, versionBefore(t, dir, "_add_classes_teacher.sql")); err != nil {
		t.Fatalf("up to the class teacher: %v", err)
	}
	var admin, class string
	if err := migrate.QueryRow(`INSERT INTO app.users (email, full_name, role) VALUES ('owner@example.com', 'Owner', 'admin') RETURNING id::text`).Scan(&admin); err != nil {
		t.Fatal(err)
	}
	if err := migrate.QueryRow(`INSERT INTO app.classes (name) VALUES ('Lớp') RETURNING id::text`).Scan(&class); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	app := openAs(t, appRoleDSN(t, dsn))
	old, err := app.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = old.Rollback() }()
	var student string
	if err := old.QueryRowContext(ctx, `INSERT INTO app.users (email, full_name) VALUES ('new@example.com', 'New') RETURNING id::text`).Scan(&student); err != nil {
		t.Fatalf("the old binary's student insert: %v", err)
	}

	column := make(chan error, 1)
	go func() { column <- goose.UpByOne(migrate, dir) }()
	select {
	case err := <-column:
		if err != nil {
			t.Fatalf("the class teacher column: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("the class teacher column waited on the old binary's open student insert: it locks app.users while holding app.classes")
	}
	if _, err := old.ExecContext(ctx, `INSERT INTO app.class_members (class_id, user_id, joined_via, added_by) VALUES ($1::uuid, $2::uuid, 'admin', $3::uuid)`, class, student, admin); err != nil {
		t.Fatalf("the old binary's enrolment after the column: %v", err)
	}

	keys := make(chan error, 1)
	go func() { keys <- goose.Up(migrate, dir) }()
	select {
	case err := <-keys:
		t.Fatalf("the foreign keys finished while the old binary still held app.users: %v", err)
	case <-time.After(2 * time.Second):
	}
	if err := old.Commit(); err != nil {
		t.Fatalf("the old binary's commit: %v", err)
	}
	if err := <-keys; err != nil {
		t.Fatalf("the rest of the migrations after the old binary committed: %v", err)
	}
	var teacher string
	var restrict bool
	if err := migrate.QueryRow(`
		SELECT c.teacher_id::text,
		       (SELECT k.confdeltype = 'r' AND k.convalidated FROM pg_constraint k WHERE k.conname = 'classes_teacher_id_fkey')
		  FROM app.classes c WHERE c.id = $1`, class).Scan(&teacher, &restrict); err != nil {
		t.Fatal(err)
	}
	if teacher != admin || !restrict {
		t.Errorf("the class's teacher is %s (want %s) and its key restricts: %v", teacher, admin, restrict)
	}
}
