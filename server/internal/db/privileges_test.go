package db_test

import (
	"database/sql"
	"testing"
)

// Two properties of this system are enforced by PostgreSQL PRIVILEGES rather
// than by schema or code, and until now nothing asserted either one:
//
//  1. New tables become readable and writable by the app role automatically,
//     via `ALTER DEFAULT PRIVILEGES` in 00009 -- which is what lets every
//     Phase 2-5 migration add a table without remembering to grant anything.
//  2. audit_log is append-only, via the REVOKE at the end of 00009. audit.go
//     describes the table as "append-only by PRIVILEGE, not by convention".
//
// Every integration test connects as the OWNER, which bypasses privilege checks
// entirely -- so the role the tests exercise and the role production uses were
// different, and the difference was invisible to CI.
//
// That is not hypothetical here. 00009's own comment records this class of
// failure happening once already: "the first integration test to connect as the
// app role failed with 'permission denied for schema app'". It was caught then
// because something happened to connect as the app role. Nothing did afterwards.
//
// These ask the database about another role's privileges rather than opening a
// second connection, so they need no extra DSN and no extra CI wiring: the
// owner can interrogate quizzivy_app's grants directly.

const appRole = "quizzivy_app"

// TestAppRoleCanReadAndWriteEveryTable loops over information_schema rather than
// naming tables, which is the same property ALTER DEFAULT PRIVILEGES was chosen
// for: a table added later is covered without anyone extending this test.
func TestAppRoleCanReadAndWriteEveryTable(t *testing.T) {
	conn := migrated(t)

	rows, err := conn.Query(`
		SELECT table_name
		  FROM information_schema.tables
		 WHERE table_schema = 'app' AND table_type = 'BASE TABLE'
		 ORDER BY table_name`)
	if err != nil {
		t.Fatalf("listing tables: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(tables) == 0 {
		t.Fatal("no tables in schema app; migrations did not run")
	}

	for _, table := range tables {
		for _, priv := range []string{"SELECT", "INSERT"} {
			if !hasTablePrivilege(t, conn, appRole, "app."+table, priv) {
				t.Errorf("%s cannot %s app.%s -- ALTER DEFAULT PRIVILEGES did not cover it, "+
					"so production would fail every query against this table",
					appRole, priv, table)
			}
		}
	}
	t.Logf("checked SELECT and INSERT for %s on %d tables", appRole, len(tables))
}

// TestTheAppendOnlyTablesAreAppendOnlyForTheAppRole makes §13.4's claim a
// privilege rather than a promise. A record the application can rewrite is not
// evidence, and both of these tables exist to be evidence: audit_log is what
// the teacher's actions are reconstructed from, attempt_events is what a
// student's session is.
//
// The REVOKE is what makes it true, and it lives beside each table -- audit_log
// in 00009, attempt_events in 00023 -- so a table is never briefly writable
// between being created and being locked down.
func TestTheAppendOnlyTablesAreAppendOnlyForTheAppRole(t *testing.T) {
	conn := migrated(t)

	for _, table := range []string{"app.audit_log", "app.attempt_events"} {
		t.Run(table, func(t *testing.T) {
			for _, priv := range []string{"SELECT", "INSERT"} {
				if !hasTablePrivilege(t, conn, appRole, table, priv) {
					t.Errorf("%s cannot %s %s; it has to be able to write entries", appRole, priv, table)
				}
			}
			for _, priv := range []string{"UPDATE", "DELETE"} {
				if hasTablePrivilege(t, conn, appRole, table, priv) {
					t.Errorf("%s can %s %s -- append-only is the point, and the REVOKE "+
						"in the migration that creates it is what makes that true", appRole, priv, table)
				}
			}
		})
	}
}

// TestAppRoleOwnsNothing is the other half of §13.5: the app connects with DML
// only and can never run DDL, so a compromised application cannot alter the
// schema it is constrained by.
func TestAppRoleOwnsNothing(t *testing.T) {
	conn := migrated(t)

	var owned int
	if err := conn.QueryRow(`
		SELECT count(*) FROM pg_tables
		 WHERE schemaname = 'app' AND tableowner = $1`, appRole).Scan(&owned); err != nil {
		t.Fatalf("checking ownership: %v", err)
	}
	if owned != 0 {
		t.Errorf("%s owns %d tables in schema app; it should own none (§13.5)", appRole, owned)
	}

	var canCreate bool
	if err := conn.QueryRow(
		`SELECT has_schema_privilege($1, 'app', 'CREATE')`, appRole).Scan(&canCreate); err != nil {
		t.Fatalf("checking schema privilege: %v", err)
	}
	if canCreate {
		t.Errorf("%s has CREATE on schema app; it should have USAGE only (§13.5)", appRole)
	}
}

func hasTablePrivilege(t *testing.T, conn *sql.DB, role, table, priv string) bool {
	t.Helper()
	var ok bool
	if err := conn.QueryRow(
		`SELECT has_table_privilege($1, $2, $3)`, role, table, priv).Scan(&ok); err != nil {
		t.Fatalf("has_table_privilege(%s, %s, %s): %v", role, table, priv, err)
	}
	return ok
}
