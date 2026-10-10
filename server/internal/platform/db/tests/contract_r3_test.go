//go:build integration

package db_test

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pressly/goose/v3"

	"quizzivy/internal/platform/db"
)

const dropLegacyRoleSuffix = "_drop_users_legacy_role.sql"

func contractDatabase(t *testing.T) (*sql.DB, string) {
	t.Helper()
	if os.Getenv("TEST_DESTRUCTIVE") != "1" {
		t.Skip("TEST_DESTRUCTIVE=1 not set; skipping the contract test, which creates and drops a scratch database")
	}
	nonce := make([]byte, 6)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	name := "qv_contract_" + hex.EncodeToString(nonce)
	admin, err := sql.Open("pgx", db.TestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	t.Cleanup(func() {
		if _, err := admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`); err != nil {
			t.Errorf("dropping %s: %v", name, err)
		}
	})
	if _, err := admin.Exec(`CREATE DATABASE ` + name); err != nil {
		t.Fatalf("creating %s: %v", name, err)
	}
	conn, err := sql.Open("pgx", swapDatabase(t, db.TestDSN(t), name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	goose.SetLogger(goose.NopLogger())
	return conn, db.MigrationsDir(t)
}

func versionOfMigration(t *testing.T, dir, suffix string) int64 {
	t.Helper()
	migrations, err := goose.CollectMigrations(dir, 0, math.MaxInt64)
	if err != nil {
		t.Fatalf("collect migrations: %v", err)
	}
	for _, m := range migrations {
		if strings.HasSuffix(filepath.Base(m.Source), suffix) {
			return m.Version
		}
	}
	t.Fatalf("no migration ends in %s", suffix)
	return 0
}

func contractHolds(t *testing.T, conn *sql.DB, query string, args ...any) bool {
	t.Helper()
	var yes bool
	if err := conn.QueryRow(`SELECT EXISTS (`+query+`)`, args...).Scan(&yes); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return yes
}

func contractExec(t *testing.T, conn *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(query, args...); err != nil {
		t.Fatalf("%.100s: %v", query, err)
	}
}

func contractID(t *testing.T, conn *sql.DB, query string, args ...any) string {
	t.Helper()
	var id string
	if err := conn.QueryRow(query, args...).Scan(&id); err != nil {
		t.Fatalf("%.100s: %v", query, err)
	}
	return id
}

func contractPgError(t *testing.T, err error) *pgconn.PgError {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("want a database error, got %v", err)
	}
	return pgErr
}

const legacyRoleColumn = `
	SELECT 1 FROM pg_attribute WHERE attrelid = 'app.users'::regclass AND attname = 'role' AND NOT attisdropped`

func TestDroppingTheLegacyRoleLeavesNothingOfItBehind(t *testing.T) {
	conn, dir := contractDatabase(t)
	if err := goose.Up(conn, dir); err != nil {
		t.Fatalf("up: %v", err)
	}

	for name, query := range map[string]string{
		"the column users.role":               legacyRoleColumn,
		"the type app.user_role":              `SELECT 1 FROM pg_type WHERE typname = 'user_role' AND typnamespace = 'app'::regnamespace`,
		"the function users_sync_legacy_role": `SELECT 1 FROM pg_proc WHERE proname = 'users_sync_legacy_role' AND pronamespace = 'app'::regnamespace`,
		"the trigger users_sync_legacy_role":  `SELECT 1 FROM pg_trigger WHERE tgname = 'users_sync_legacy_role'`,
		"the index users_role_active_idx":     `SELECT 1 FROM pg_class WHERE relname = 'users_role_active_idx' AND relnamespace = 'app'::regnamespace`,
	} {
		if contractHolds(t, conn, query) {
			t.Errorf("%s survived the migration", name)
		}
	}
	for name, query := range map[string]string{
		"the index users_role_id_active_idx": `SELECT 1 FROM pg_class WHERE relname = 'users_role_id_active_idx' AND relnamespace = 'app'::regnamespace`,
		"the validated constraint users_role_id_not_null": `
			SELECT 1 FROM pg_constraint WHERE conname = 'users_role_id_not_null' AND conrelid = 'app.users'::regclass AND convalidated`,
		"the trigger users_last_admin":     `SELECT 1 FROM pg_trigger WHERE tgname = 'users_last_admin' AND tgenabled = 'O'`,
		"the trigger users_set_updated_at": `SELECT 1 FROM pg_trigger WHERE tgname = 'users_set_updated_at' AND tgenabled = 'O'`,
	} {
		if !contractHolds(t, conn, query) {
			t.Errorf("%s is missing", name)
		}
	}

	_, err := conn.Exec(`INSERT INTO app.users (email, full_name) VALUES ('norole@example.com', 'No Role')`)
	if err == nil {
		t.Fatal("a user without a role_id was accepted")
	}
	if pgErr := contractPgError(t, err); pgErr.Code != "23502" || pgErr.ColumnName != "role_id" {
		t.Errorf("a user without a role_id: %s on column %q, want 23502 on role_id", pgErr.Code, pgErr.ColumnName)
	}
	contractExec(t, conn, `INSERT INTO app.users (email, full_name, role_id)
		VALUES ('withrole@example.com', 'With Role', (SELECT id FROM app.roles WHERE builtin_key = 'student'))`)
}

func TestTheLegacyRoleMigrationStopsAtAUserWithoutARole(t *testing.T) {
	conn, dir := contractDatabase(t)
	before := versionBefore(t, dir, dropLegacyRoleSuffix)
	if err := goose.UpTo(conn, dir, before); err != nil {
		t.Fatalf("up to the expand half: %v", err)
	}
	contractExec(t, conn, `ALTER TABLE app.users DROP CONSTRAINT users_role_id_not_null`)
	contractExec(t, conn, `ALTER TABLE app.users DISABLE TRIGGER users_sync_legacy_role`)
	contractExec(t, conn, `INSERT INTO app.users (email, full_name) VALUES ('orphan@example.com', 'Orphan')`)
	contractExec(t, conn, `ALTER TABLE app.users ENABLE TRIGGER users_sync_legacy_role`)
	contractExec(t, conn, `ALTER TABLE app.users ADD CONSTRAINT users_role_id_not_null NOT NULL role_id NOT VALID`)

	err := goose.UpByOne(conn, dir)
	if err == nil {
		t.Fatal("the migration ran over a user without a role")
	}
	if pgErr := contractPgError(t, err); pgErr.Code != "23502" || pgErr.ColumnName != "role_id" {
		t.Errorf("stopped by %s on column %q, want 23502 on role_id", pgErr.Code, pgErr.ColumnName)
	}
	if !contractHolds(t, conn, legacyRoleColumn) {
		t.Error("the column went although the migration failed")
	}
	if !contractHolds(t, conn, `SELECT 1 FROM pg_trigger WHERE tgname = 'users_sync_legacy_role'`) {
		t.Error("the trigger went although the migration failed")
	}
	if version, err := goose.GetDBVersion(conn); err != nil || version != before {
		t.Errorf("version = %d (%v), want it left at %d", version, err, before)
	}
}

func TestTheLegacyRoleMigrationGivesUpWhenItsLockDoesNotArrive(t *testing.T) {
	conn, dir := contractDatabase(t)
	before := versionBefore(t, dir, dropLegacyRoleSuffix)
	if err := goose.UpTo(conn, dir, before); err != nil {
		t.Fatalf("up to the expand half: %v", err)
	}
	reader, err := conn.Begin()
	if err != nil {
		t.Fatal(err)
	}
	released := false
	release := func() {
		if released {
			return
		}
		released = true
		if err := reader.Rollback(); err != nil {
			t.Errorf("releasing the reader: %v", err)
		}
	}
	t.Cleanup(release)
	if _, err := reader.Exec(`SELECT 1 FROM app.users LIMIT 1`); err != nil {
		t.Fatal(err)
	}

	result := make(chan error, 1)
	go func() { result <- goose.UpByOne(conn, dir) }()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("the migration ran while a query held app.users")
		}
		if pgErr := contractPgError(t, err); pgErr.Code != "55P03" {
			t.Errorf("stopped by %s, want 55P03 (lock_not_available)", pgErr.Code)
		}
	case <-time.After(30 * time.Second):
		release()
		t.Fatal("the migration kept waiting for its lock past its lock_timeout")
	}
	release()
	if !contractHolds(t, conn, legacyRoleColumn) {
		t.Error("the column went although the migration gave up")
	}
	if version, err := goose.GetDBVersion(conn); err != nil || version != before {
		t.Errorf("version = %d (%v), want it left at %d", version, err, before)
	}
}

func TestTheLegacyRoleComesBackFromWhatTheRoleHoldsAndGoesAgain(t *testing.T) {
	conn, dir := contractDatabase(t)
	removed := versionOfMigration(t, dir, dropLegacyRoleSuffix)
	if err := goose.UpTo(conn, dir, removed); err != nil {
		t.Fatalf("up: %v", err)
	}

	contractExec(t, conn, `INSERT INTO app.role_permissions (role_id, permission_key)
		SELECT id, 'learning.take_tests' FROM app.roles WHERE builtin_key = 'admin'`)
	readerOnly := contractID(t, conn, `INSERT INTO app.roles (name, icon, color) VALUES ('Chỉ làm bài', 'user', 'gray') RETURNING id::text`)
	contractExec(t, conn, `INSERT INTO app.role_permissions (role_id, permission_key) VALUES ($1, 'learning.take_tests')`, readerOnly)
	editor := contractID(t, conn, `INSERT INTO app.roles (name, icon, color) VALUES ('Biên tập', 'user', 'gray') RETURNING id::text`)
	contractExec(t, conn, `INSERT INTO app.role_permissions (role_id, permission_key) VALUES ($1, 'learning.take_tests'), ($1, 'content.tests.write')`, editor)

	want := map[string]string{
		"admin@example.com":       "admin",
		"teacher@example.com":     "admin",
		"assistant@example.com":   "admin",
		"student@example.com":     "student",
		"reader-only@example.com": "student",
		"editor-also@example.com": "admin",
	}
	for email, key := range map[string]string{
		"admin@example.com":     "admin",
		"teacher@example.com":   "teacher",
		"assistant@example.com": "assistant",
		"student@example.com":   "student",
	} {
		contractExec(t, conn, `INSERT INTO app.users (email, full_name, role_id, created_at, updated_at)
			VALUES ($1, $1, (SELECT id FROM app.roles WHERE builtin_key = $2), now() - interval '30 days', now() - interval '30 days')`, email, key)
	}
	contractExec(t, conn, `INSERT INTO app.users (email, full_name, role_id, created_at, updated_at)
		VALUES ('reader-only@example.com', 'r', $1, now() - interval '30 days', now() - interval '30 days')`, readerOnly)
	contractExec(t, conn, `INSERT INTO app.users (email, full_name, role_id, created_at, updated_at)
		VALUES ('editor-also@example.com', 'e', $1, now() - interval '30 days', now() - interval '30 days')`, editor)
	stamps := userStamps(t, conn)
	contractExec(t, conn, `ALTER DEFAULT PRIVILEGES IN SCHEMA app REVOKE USAGE ON TYPES FROM quizzivy_app`)
	contractExec(t, conn, `ALTER DEFAULT PRIVILEGES REVOKE USAGE ON TYPES FROM PUBLIC`)

	if err := goose.Down(conn, dir); err != nil {
		t.Fatalf("down: %v", err)
	}

	rows, err := conn.Query(`SELECT email, role::text FROM app.users`)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for rows.Next() {
		var email, role string
		if err := rows.Scan(&email, &role); err != nil {
			t.Fatal(err)
		}
		got[email] = role
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	for email, role := range want {
		if got[email] != role {
			t.Errorf("%s comes back as %q, want %q", email, got[email], role)
		}
	}
	if after := userStamps(t, conn); after != stamps {
		t.Errorf("the backfill moved updated_at:\nbefore %s\nafter  %s", stamps, after)
	}
	assertLegacyRoleRestored(t, conn)

	if err := goose.UpTo(conn, dir, removed); err != nil {
		t.Fatalf("up again: %v", err)
	}
	if contractHolds(t, conn, legacyRoleColumn) {
		t.Error("the column survived the second up")
	}
}

func userStamps(t *testing.T, conn *sql.DB) string {
	t.Helper()
	var stamps string
	if err := conn.QueryRow(`SELECT coalesce(string_agg(email || '=' || updated_at::text, ',' ORDER BY email), '') FROM app.users`).Scan(&stamps); err != nil {
		t.Fatal(err)
	}
	return stamps
}

func assertLegacyRoleRestored(t *testing.T, conn *sql.DB) {
	t.Helper()
	if !contractHolds(t, conn, `SELECT 1 WHERE has_type_privilege('quizzivy_app', 'app.user_role', 'USAGE')`) {
		t.Error("the app role has no USAGE on app.user_role")
	}
	if !contractHolds(t, conn, `
		SELECT 1 FROM pg_attribute a JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
		 WHERE a.attrelid = 'app.users'::regclass AND a.attname = 'role' AND a.attnotnull
		   AND pg_get_expr(d.adbin, d.adrelid) = '''student''::app.user_role'`) {
		t.Error("role is not NOT NULL DEFAULT 'student'")
	}
	if !contractHolds(t, conn, `SELECT 1 FROM pg_class WHERE relname = 'users_role_active_idx' AND relnamespace = 'app'::regnamespace`) {
		t.Error("users_role_active_idx is missing")
	}
	if !contractHolds(t, conn, `SELECT 1 FROM pg_constraint WHERE conname = 'users_role_id_not_null' AND convalidated`) {
		t.Error("Down weakened users_role_id_not_null")
	}
	if contractHolds(t, conn, `SELECT 1 FROM pg_trigger WHERE NOT tgisinternal AND tgenabled <> 'O'`) {
		t.Error("a trigger was left disabled")
	}
	derived := contractID(t, conn, `
		WITH ins AS (INSERT INTO app.users (email, full_name, role) VALUES ('old-binary@example.com', 'Old', 'admin') RETURNING role_id)
		SELECT r.builtin_key FROM ins JOIN app.roles r ON r.id = ins.role_id`)
	if derived != "admin" {
		t.Errorf("an insert with only role gets the %s role, want admin", derived)
	}
	contractExec(t, conn, `DELETE FROM app.users WHERE email = 'old-binary@example.com'`)
}
