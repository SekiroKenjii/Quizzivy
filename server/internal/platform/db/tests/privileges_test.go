package db_test

import (
	"database/sql"
	"testing"
)

// Two properties of this system are enforced by PostgreSQL PRIVILEGES rather
// than by schema or code, and until now nothing asserted either one:

const appRole = "quizzivy_app"

var readOnlyForApp = map[string]bool{
	"maintenance_windows": true,
	"permissions":         true,
}

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
		privileges := []string{"SELECT", "INSERT"}
		if readOnlyForApp[table] {
			privileges = []string{"SELECT"}
		}
		for _, priv := range privileges {
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
func TestTheAppendOnlyTablesAreAppendOnlyForTheAppRole(t *testing.T) {
	conn := migrated(t)

	for _, table := range []string{"app.audit_log", "app.attempt_events", "app.attempt_group_audio_receipts", "app.ai_usage_ledger"} {
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

func TestTheAppRoleOnlyReadsMaintenanceWindows(t *testing.T) {
	conn := migrated(t)

	if !hasTablePrivilege(t, conn, appRole, "app.maintenance_windows", "SELECT") {
		t.Errorf("%s cannot SELECT app.maintenance_windows; the API reads windows", appRole)
	}
	for _, priv := range []string{"INSERT", "UPDATE", "DELETE"} {
		if hasTablePrivilege(t, conn, appRole, "app.maintenance_windows", priv) {
			t.Errorf("%s can %s app.maintenance_windows; only the operator writes windows", appRole, priv)
		}
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
		`SELECT has_schema_privilege($1, 'app', 'CREATE')`, appRole,
	).Scan(&canCreate); err != nil {
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
		`SELECT has_table_privilege($1, $2, $3)`, role, table, priv,
	).Scan(&ok); err != nil {
		t.Fatalf("has_table_privilege(%s, %s, %s): %v", role, table, priv, err)
	}
	return ok
}

func TestTheAppRoleCannotCreateTemporaryObjects(t *testing.T) {
	conn := migrated(t)
	var allowed bool
	if err := conn.QueryRow(
		`SELECT has_database_privilege($1, current_database(), 'TEMPORARY')`, appRole).Scan(&allowed); err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Errorf("%s may create temporary objects in this database; migration 00079 revokes that only where the migration role owns the database", appRole)
	}
}

// TestTheAppRoleWritesRolesAndGrantsAndNothingElseOfTheAccessModel pins
// 00129_grant_role_writes: the role commands insert roles, change the columns
// a role command changes, and add and remove grants. A role is never deleted
// (DG-52), a built-in key never changes, and the permission catalogue is the
// migration role's alone.
func TestTheAppRoleWritesRolesAndGrantsAndNothingElseOfTheAccessModel(t *testing.T) {
	conn := migrated(t)

	for _, c := range []struct {
		table string
		priv  string
		want  bool
	}{
		{"app.roles", "INSERT", true},
		{"app.roles", "DELETE", false},
		{"app.roles", "UPDATE", false},
		{"app.role_permissions", "INSERT", true},
		{"app.role_permissions", "DELETE", true},
		{"app.role_permissions", "UPDATE", false},
		{"app.permissions", "INSERT", false},
		{"app.permissions", "UPDATE", false},
		{"app.permissions", "DELETE", false},
	} {
		if got := hasTablePrivilege(t, conn, appRole, c.table, c.priv); got != c.want {
			t.Errorf("%s %s on %s = %v, want %v", appRole, c.priv, c.table, got, c.want)
		}
	}
	for column, want := range map[string]bool{
		"name": true, "description": true, "icon": true, "color": true, "revision": true, "updated_at": true,
		"id": false, "builtin_key": false, "copied_from": false, "created_by": false, "created_at": false,
	} {
		var got bool
		if err := conn.QueryRow(`SELECT has_column_privilege($1, 'app.roles', $2, 'UPDATE')`, appRole, column).Scan(&got); err != nil {
			t.Fatalf("has_column_privilege(%s): %v", column, err)
		}
		if got != want {
			t.Errorf("%s UPDATE on app.roles(%s) = %v, want %v", appRole, column, got, want)
		}
	}
}
