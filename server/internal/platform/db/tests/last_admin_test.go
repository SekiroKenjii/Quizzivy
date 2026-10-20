//go:build integration

package db_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pressly/goose/v3"

	identitydomain "quizzivy/internal/modules/identity/domain"
	identityrepo "quizzivy/internal/modules/identity/repositories"
	"quizzivy/internal/platform/db"
)

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

func migratedScratch(t *testing.T) (string, *sql.DB) {
	t.Helper()
	if os.Getenv("TEST_DESTRUCTIVE") != "1" {
		t.Skip("TEST_DESTRUCTIVE=1 not set; skipping the last-Admin guard test, which creates and drops a scratch database")
	}
	nonce := make([]byte, 6)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	name := "qv_last_admin_" + hex.EncodeToString(nonce)
	admin, err := sql.Open("pgx", db.TestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	t.Cleanup(func() { dropScratchDatabase(t, admin, name) })
	if _, err := admin.Exec(`CREATE DATABASE ` + name); err != nil {
		t.Fatalf("creating %s: %v", name, err)
	}
	dsn := swapDatabase(t, db.TestDSN(t), name)
	migrate := openAs(t, dsn)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	goose.SetLogger(goose.NopLogger())
	if err := goose.Up(migrate, db.MigrationsDir(t)); err != nil {
		t.Fatalf("up: %v", err)
	}
	return dsn, migrate
}

func TestTheLastActiveAdminCannotLeave(t *testing.T) {
	dsn, migrate := migratedScratch(t)
	user := func(email, builtin string) string {
		t.Helper()
		var id string
		if err := migrate.QueryRow(`
			INSERT INTO app.users (email, full_name, role_id)
			VALUES ($1, $1, (SELECT id FROM app.roles WHERE builtin_key = $2))
			RETURNING id::text`, email, builtin).Scan(&id); err != nil {
			t.Fatalf("inserting %s: %v", email, err)
		}
		return id
	}
	owner := user("owner@example.com", "admin")
	secondAdmin := user("second-admin@example.com", "admin")
	student := user("minh@example.com", "student")
	promotable := user("an@example.com", "student")
	app := openAs(t, appRoleDSN(t, dsn))

	if _, err := app.Exec(`UPDATE app.users SET disabled_at = now() WHERE id = $1`, secondAdmin); err != nil {
		t.Fatalf("disabling one of two admins: %v", err)
	}

	t.Run("each way out is refused and maps to ErrLastAdmin", func(t *testing.T) {
		for name, statement := range map[string]string{
			"demote":             `UPDATE app.users SET role_id = (SELECT id FROM app.roles WHERE builtin_key = 'teacher') WHERE id = $1`,
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
				if !errors.Is(identityrepo.UserWriteError(err), identitydomain.ErrLastAdmin) {
					t.Errorf("the identity repository maps the refusal to %v, want ErrLastAdmin", identityrepo.UserWriteError(err))
				}
			})
		}
	})

	t.Run("a student change takes no lock on the Admin role", func(t *testing.T) {
		tx, err := app.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		if _, err := tx.Exec(`UPDATE app.users SET disabled_at = now() WHERE id = $1`, student); err != nil {
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
		if _, err := migrate.Exec(`UPDATE app.users SET disabled_at = NULL WHERE id = $1`, secondAdmin); err != nil {
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
			_, err := second.ExecContext(ctx, `UPDATE app.users SET disabled_at = now() WHERE id = $1`, secondAdmin)
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
			[]string{owner, secondAdmin}); err != nil {
			t.Fatal(err)
		}
		promoting, err := app.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = promoting.Rollback() }()
		if _, err := promoting.Exec(`UPDATE app.users SET role_id = (SELECT id FROM app.roles WHERE builtin_key = 'admin') WHERE id = $1`, promotable); err != nil {
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
		if _, err := disabling.Exec(`UPDATE app.users SET disabled_at = now() WHERE id = $1`, secondAdmin); err != nil {
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
