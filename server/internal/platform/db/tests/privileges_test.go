package db_test

import (
	"database/sql"
	"testing"
)

// Two properties of this system are enforced by PostgreSQL PRIVILEGES rather
// than by schema or code, and until now nothing asserted either one:

const appRole = "quizzivy_app"

var readOnlyForApp = map[string]bool{
	"maintenance_windows":   true,
	"permissions":           true,
	"organization_settings": true,
}

func TestTheOrganizationSettingsRowCanBeChangedButNeitherAddedNorRemovedByTheAppRole(t *testing.T) {
	conn := migrated(t)
	for priv, want := range map[string]bool{"SELECT": true, "UPDATE": true, "INSERT": false, "DELETE": false} {
		if got := hasTablePrivilege(t, conn, appRole, "app.organization_settings", priv); got != want {
			t.Errorf("%s %s on app.organization_settings = %v, want %v", appRole, priv, got, want)
		}
	}
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

// TestTheAppRoleCannotRewriteTheFrozenRubricsAndMatchItems keeps R6's two
// frozen tables as frozen as the rest of a published version: publish inserts
// them and nothing changes them afterwards.
func TestTheAppRoleCannotRewriteTheFrozenRubricsAndMatchItems(t *testing.T) {
	conn := migrated(t)
	for _, table := range []string{"app.test_version_rubric_criteria", "app.test_version_match_items"} {
		for priv, want := range map[string]bool{"SELECT": true, "INSERT": true, "UPDATE": false, "DELETE": true} {
			if got := hasTablePrivilege(t, conn, appRole, table, priv); got != want {
				t.Errorf("%s %s on %s = %v, want %v", appRole, priv, table, got, want)
			}
		}
	}
	// The editor rewrites the bank rows and a regrade replaces criterion grades
	// (QC-R6B-2), so those tables keep UPDATE and DELETE.
	for _, table := range []string{"app.question_rubric_criteria", "app.question_match_items", "app.attempt_answer_criteria"} {
		for _, priv := range []string{"SELECT", "INSERT", "UPDATE", "DELETE"} {
			if !hasTablePrivilege(t, conn, appRole, table, priv) {
				t.Errorf("%s cannot %s %s", appRole, priv, table)
			}
		}
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

const maintenanceRole = "quizzivy_maintenance"

var maintenanceGrants = map[string]map[string]string{
	"ai_credit_periods":            {"SELECT": "*", "INSERT": "*", "UPDATE": "*"},
	"ai_credit_requests":           {"SELECT": "*", "UPDATE": "*"},
	"ai_reservations":              {"SELECT": "*", "DELETE": "*"},
	"ai_settings":                  {"SELECT": "*", "UPDATE": "*"},
	"ai_teacher_credits":           {"SELECT": "*", "INSERT": "*", "UPDATE": "*"},
	"assignment_student_overrides": {"SELECT": "assignment_id,closes_at,student_id", "UPDATE": "closes_at"},
	"assignments":                  {"SELECT": "closed_at,closes_at,created_by,id,published_at", "UPDATE": "closes_at"},
	"attempt_answers":              {"SELECT": "attempt_id,payload,question_id", "UPDATE": "payload"},
	"attempt_events":               {"SELECT": "attempt_id,id,received_at", "DELETE": "*"},
	"attempts":                     {"SELECT": "assignment_id,deadline_at,id,status,student_id,submitted_at", "UPDATE": "deadline_at"},
	"audit_log":                    {"INSERT": "*"},
	"class_join_codes":             {"SELECT": "class_id,code_ciphertext,id,key_id,lookup_scheme,revoked_at", "UPDATE": "code_ciphertext,code_hash,key_id"},
	"classes":                      {"SELECT": "teacher_id"},
	"maintenance_windows":          {"SELECT": "*", "INSERT": "ends_at,starts_at", "UPDATE": "cancelled_at,ends_at"},
	"media_assets":                 {"SELECT": "owner_id"},
	"notification_preferences":     {"SELECT": "event,in_app,user_id"},
	"notifications": {
		"SELECT": "dedupe_key,id,kind,params,target,user_id",
		"INSERT": "dedupe_key,kind,params,target,user_id",
		"UPDATE": "kind,params,read_at,target",
		"DELETE": "*",
	},
	"organization_settings": {"SELECT": "anonymise_disabled_after_months,keep_answers_months,keep_focus_events_months"},
	"question_groups":       {"SELECT": "owner_id"},
	"questions":             {"SELECT": "owner_id"},
	"refresh_tokens":        {"SELECT": "user_id", "DELETE": "*"},
	"role_permissions":      {"SELECT": "*"},
	"roles":                 {"SELECT": "*"},
	"set_password_tokens":   {"SELECT": "revoked_at,used_at,user_id", "UPDATE": "revoked_at"},
	"student_like_roles":    {"SELECT": "*"},
	"tests":                 {"SELECT": "owner_id"},
	"user_identities":       {"SELECT": "user_id", "DELETE": "*"},
	"users": {
		"SELECT": "disabled_at,display_name,email,full_name,id,role_id",
		"UPDATE": "avatar_key,disabled_at,display_name,email,full_name,must_change_password,password_hash,phone",
	},
}

const maintenanceGrantsQuery = `
	WITH cols AS (
	  SELECT c.oid, c.relname, a.attnum, a.attname, count(*) OVER (PARTITION BY c.oid) AS total
	    FROM pg_class c
	    JOIN pg_namespace n ON n.oid = c.relnamespace
	    JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum > 0 AND NOT a.attisdropped
	   WHERE n.nspname = 'app' AND c.relkind IN ('r', 'p', 'v', 'm', 'f')
	), held AS (
	  SELECT cols.relname, p.priv, cols.attname, cols.total
	    FROM cols
	   CROSS JOIN unnest(ARRAY['SELECT', 'INSERT', 'UPDATE', 'REFERENCES']) AS p(priv)
	   WHERE has_column_privilege($1, cols.oid, cols.attnum, p.priv)
	)
	SELECT relname, priv, CASE WHEN count(*) = max(total) THEN '*' ELSE string_agg(attname, ',' ORDER BY attname) END
	  FROM held GROUP BY relname, priv
	UNION ALL
	SELECT c.relname, p.priv, '*'
	  FROM pg_class c
	  JOIN pg_namespace n ON n.oid = c.relnamespace
	 CROSS JOIN unnest(ARRAY['DELETE', 'TRUNCATE', 'TRIGGER']) AS p(priv)
	 WHERE n.nspname = 'app' AND c.relkind IN ('r', 'p') AND has_table_privilege($1, c.oid, p.priv)`

func TestTheMaintenanceRoleHoldsExactlyTheGrantsItsCommandsUse(t *testing.T) {
	conn := migrated(t)
	rows, err := conn.Query(maintenanceGrantsQuery, maintenanceRole)
	if err != nil {
		t.Fatalf("reading %s's grants: %v", maintenanceRole, err)
	}
	defer func() { _ = rows.Close() }()
	got := map[string]string{}
	for rows.Next() {
		var table, priv, columns string
		if err := rows.Scan(&table, &priv, &columns); err != nil {
			t.Fatal(err)
		}
		got[table+" "+priv] = columns
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{}
	for table, privileges := range maintenanceGrants {
		for priv, columns := range privileges {
			want[table+" "+priv] = columns
		}
	}
	for key, columns := range want {
		if got[key] != columns {
			t.Errorf("%s on %s = %q, want %q", maintenanceRole, key, got[key], columns)
		}
	}
	for key, columns := range got {
		if _, ok := want[key]; !ok {
			t.Errorf("%s holds %s (%s) that no command uses; a grant is added to this list in the migration that needs it", maintenanceRole, key, columns)
		}
	}
}

func TestTheMaintenanceRoleCannotRewriteARecordOrCreateAnything(t *testing.T) {
	conn := migrated(t)

	for _, table := range []string{"app.audit_log", "app.attempt_events", "app.attempt_group_audio_receipts", "app.ai_usage_ledger"} {
		for _, priv := range []string{"UPDATE", "TRUNCATE"} {
			if hasTablePrivilege(t, conn, maintenanceRole, table, priv) {
				t.Errorf("%s can %s %s; a record the application can rewrite is not evidence", maintenanceRole, priv, table)
			}
		}
	}
	if hasTablePrivilege(t, conn, maintenanceRole, "app.audit_log", "DELETE") {
		t.Errorf("%s can DELETE app.audit_log; the audit log is kept for ever", maintenanceRole)
	}
	var ledger bool
	if err := conn.QueryRow(`SELECT has_any_column_privilege($1, 'app.ai_usage_ledger', 'SELECT, INSERT, UPDATE, REFERENCES')
		OR has_table_privilege($1, 'app.ai_usage_ledger', 'DELETE')`, maintenanceRole).Scan(&ledger); err != nil {
		t.Fatal(err)
	}
	if ledger {
		t.Errorf("%s holds a privilege on app.ai_usage_ledger; the ledger is the provider's invoice and nothing here needs it", maintenanceRole)
	}

	for _, c := range []struct {
		query string
		want  bool
		what  string
	}{
		{`SELECT has_schema_privilege($1, 'app', 'USAGE')`, true, "USAGE on schema app"},
		{`SELECT has_schema_privilege($1, 'app', 'CREATE')`, false, "CREATE on schema app"},
		{`SELECT has_schema_privilege($1, 'public', 'USAGE')`, true, "USAGE on schema public, which app.immutable_unaccent reaches"},
		{`SELECT has_schema_privilege($1, 'public', 'CREATE')`, false, "CREATE on schema public"},
		{`SELECT has_database_privilege($1, current_database(), 'CONNECT')`, true, "CONNECT on the database"},
		{`SELECT has_database_privilege($1, current_database(), 'CREATE')`, false, "CREATE on the database"},
		{`SELECT has_database_privilege($1, current_database(), 'TEMPORARY')`, false, "TEMPORARY on the database"},
		{`SELECT NOT coalesce(bool_or(CASE WHEN c.relkind = 'S' THEN has_sequence_privilege($1, c.oid, 'USAGE, SELECT, UPDATE') END), false)
		    FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname IN ('app', 'public')`, true, "no privilege on any sequence"},
		{`SELECT count(*) = 0 FROM pg_shdepend d JOIN pg_roles r ON r.oid = d.refobjid
		   WHERE r.rolname = $1 AND d.deptype = 'o'`, true, "owning no object"},
		{`SELECT count(*) = 0 FROM pg_auth_members m JOIN pg_roles r ON r.oid = m.member WHERE r.rolname = $1`, true, "membership of no role"},
		{`SELECT EXISTS (SELECT 1 FROM pg_trigger t
		     WHERE t.tgrelid = 'app.users'::regclass AND t.tgname = 'users_maintenance_guard' AND t.tgenabled = 'O'
		       AND pg_get_triggerdef(t.oid) LIKE 'CREATE TRIGGER users_maintenance_guard BEFORE UPDATE ON app.users FOR EACH ROW WHEN%'
		       AND pg_get_triggerdef(t.oid) LIKE '%' || $1 || '%')`, true, "a BEFORE UPDATE guard on app.users that fires for the role alone"},
		{`SELECT NOT has_function_privilege('public', 'app.users_maintenance_guard()', 'EXECUTE') AND NOT has_function_privilege($1, 'app.users_maintenance_guard()', 'EXECUTE')`, true, "no EXECUTE on the users guard, which fires without it"},
		{`SELECT coalesce(NOT p.prosecdef AND p.proconfig = ARRAY['search_path=pg_catalog, pg_temp']
		       AND p.proowner <> (SELECT oid FROM pg_roles WHERE rolname = $1), false)
		    FROM pg_proc p WHERE p.oid = 'app.users_maintenance_guard()'::regprocedure`, true, "a SECURITY INVOKER users guard, not owned by the role, with search_path pinned to pg_catalog, pg_temp"},
		{`SELECT NOT has_parameter_privilege($1, 'session_replication_role', 'SET')
		     AND NOT EXISTS (SELECT 1 FROM pg_parameter_acl a
		                      WHERE has_parameter_privilege($1, a.parname, 'SET') OR has_parameter_privilege($1, a.parname, 'ALTER SYSTEM'))`, true, "no right to set a parameter, session_replication_role above all, which would skip the users guard"},
		{`SELECT rolcanlogin AND rolconnlimit = 2 AND NOT (rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls)
		    FROM pg_roles WHERE rolname = $1`, true, "a login role limited to two connections with no attribute"},
	} {
		var got bool
		if err := conn.QueryRow(c.query, maintenanceRole).Scan(&got); err != nil {
			t.Fatalf("%s: %v", c.what, err)
		}
		if got != c.want {
			t.Errorf("%s: %s = %v, want %v", maintenanceRole, c.what, got, c.want)
		}
	}
}
