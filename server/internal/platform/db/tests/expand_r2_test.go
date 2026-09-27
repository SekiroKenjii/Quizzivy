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

	t.Run("the guard runs as its owner with a pinned search path", func(t *testing.T) {
		var definer bool
		var owner, config string
		if err := migrate.QueryRow(`
			SELECT p.prosecdef, pg_get_userbyid(p.proowner), coalesce(array_to_string(p.proconfig, ';'), '')
			  FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
			 WHERE n.nspname = 'app' AND p.proname = 'users_last_admin'`).Scan(&definer, &owner, &config); err != nil {
			t.Fatal(err)
		}
		if !definer || owner != "quizzivy_migrate" || config != "search_path=app, pg_catalog" {
			t.Errorf("users_last_admin: prosecdef %v, owner %s, proconfig %q", definer, owner, config)
		}
		if hasTablePrivilege(t, migrate, appRole, "app.roles", "UPDATE") {
			t.Error("quizzivy_app can UPDATE app.roles; the guard would not need to be a definer")
		}
	})
}
