package db_test

import (
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"quizzivy/internal/db"
)

// §13.7 requires every migration to have a Down that actually works. This is
// what makes that true rather than aspirational: up -> down -> up must leave a
// byte-identical schema, and the intermediate state must be genuinely empty.
//
// An unreversible migration is cheap to fix on the day it is written and
// expensive months later, when the only way to test a change is against
// production-shaped data.

func openMigrate(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := sql.Open("pgx", db.TestDSN(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := conn.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("dialect: %v", err)
	}
	goose.SetLogger(goose.NopLogger())
	return conn
}

// schemaSnapshot captures everything migrations create in `app`, ordered
// deterministically so two runs are comparable as strings.
func schemaSnapshot(t *testing.T, conn *sql.DB) string {
	t.Helper()
	const q = `
WITH types AS (
  SELECT 'type ' || t.typname || ' = ' ||
         coalesce((SELECT string_agg(e.enumlabel, ',' ORDER BY e.enumsortorder)
                   FROM pg_enum e WHERE e.enumtypid = t.oid), '') AS line
  FROM pg_type t JOIN pg_namespace n ON n.oid = t.typnamespace
  WHERE n.nspname = 'app' AND t.typtype = 'e'
),
funcs AS (
  SELECT 'func ' || p.proname || '(' || pg_get_function_identity_arguments(p.oid) || ') ' ||
         CASE p.provolatile WHEN 'i' THEN 'immutable' WHEN 's' THEN 'stable' ELSE 'volatile' END AS line
  FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
  WHERE n.nspname = 'app'
),
tables AS (
  SELECT 'table ' || c.relname || '(' ||
         (SELECT string_agg(a.attname || ':' || format_type(a.atttypid, a.atttypmod) ||
                            CASE WHEN a.attnotnull THEN ' NOT NULL' ELSE '' END, ', '
                            ORDER BY a.attnum)
          FROM pg_attribute a WHERE a.attrelid = c.oid AND a.attnum > 0 AND NOT a.attisdropped)
         || ')' AS line
  FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
  WHERE n.nspname = 'app' AND c.relkind = 'r'
),
indexes AS (
  SELECT 'index ' || indexname || ' = ' || indexdef AS line
  FROM pg_indexes WHERE schemaname = 'app'
),
constraints AS (
  SELECT 'constraint ' || c.conname || ' = ' || pg_get_constraintdef(c.oid) AS line
  FROM pg_constraint c JOIN pg_namespace n ON n.oid = c.connamespace
  WHERE n.nspname = 'app'
)
SELECT coalesce(string_agg(line, E'\n' ORDER BY line), '')
FROM (
  SELECT line FROM types
  UNION ALL SELECT line FROM funcs
  UNION ALL SELECT line FROM tables
  UNION ALL SELECT line FROM indexes
  UNION ALL SELECT line FROM constraints
) all_objects;`

	var snapshot string
	if err := conn.QueryRow(q).Scan(&snapshot); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	return snapshot
}

func TestMigrationsAreReversible(t *testing.T) {
	conn := openMigrate(t)
	dir := db.MigrationsDir(t)

	if err := goose.Up(conn, dir); err != nil {
		t.Fatalf("first up: %v", err)
	}
	before := schemaSnapshot(t, conn)
	if before == "" {
		t.Fatal("snapshot is empty after `up`; the query or the migrations are wrong")
	}

	if err := goose.Reset(conn, dir); err != nil {
		t.Fatalf("reset: %v", err)
	}

	// The intermediate state must be genuinely empty. A Down that leaves the
	// schema behind still lets up->down->up pass.
	empty := schemaSnapshot(t, conn)
	if empty != "" {
		t.Errorf("`down` left objects behind in schema app:\n%s", empty)
	}
	var schemas int
	if err := conn.QueryRow(
		`SELECT count(*) FROM information_schema.schemata WHERE schema_name = 'app'`,
	).Scan(&schemas); err != nil {
		t.Fatalf("schema check: %v", err)
	}
	if schemas != 0 {
		t.Error("schema app survived `down`")
	}

	if err := goose.Up(conn, dir); err != nil {
		t.Fatalf("second up: %v", err)
	}
	after := schemaSnapshot(t, conn)

	if before != after {
		t.Errorf("schema differs after up -> down -> up\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}
}

func TestFoundationObjectsExist(t *testing.T) {
	conn := openMigrate(t)
	if err := goose.Up(conn, db.MigrationsDir(t)); err != nil {
		t.Fatalf("up: %v", err)
	}

	var enums int
	if err := conn.QueryRow(`
		SELECT count(*) FROM pg_type t JOIN pg_namespace n ON n.oid = t.typnamespace
		WHERE n.nspname = 'app' AND t.typtype = 'e'`).Scan(&enums); err != nil {
		t.Fatal(err)
	}
	if enums != 7 {
		t.Errorf("found %d enum types in app, want 7 (§13.2 plus D-15)", enums)
	}

	for _, fn := range []string{"immutable_unaccent", "set_updated_at"} {
		var n int
		if err := conn.QueryRow(`
			SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
			WHERE n.nspname = 'app' AND p.proname = $1`, fn).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			t.Errorf("app.%s() is missing", fn)
		}
	}
}
