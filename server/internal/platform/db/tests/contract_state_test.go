//go:build integration

package db_test

import (
	"database/sql"
	"testing"
)

func catalogHolds(t *testing.T, conn *sql.DB, query string, args ...any) bool {
	t.Helper()
	var yes bool
	if err := conn.QueryRow(`SELECT EXISTS (`+query+`)`, args...).Scan(&yes); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return yes
}

func TestTheObjectsTheContractsDroppedAreGone(t *testing.T) {
	conn := migrated(t)
	gone := map[string]string{
		"the column users.role": `
			SELECT 1 FROM pg_attribute WHERE attrelid = 'app.users'::regclass AND attname = 'role' AND NOT attisdropped`,
		"the function users_sync_legacy_role": `SELECT 1 FROM pg_proc WHERE proname = 'users_sync_legacy_role' AND pronamespace = 'app'::regnamespace`,
		"the trigger users_sync_legacy_role":  `SELECT 1 FROM pg_trigger WHERE tgname = 'users_sync_legacy_role'`,
		"the index users_role_active_idx":     `SELECT 1 FROM pg_class WHERE relname = 'users_role_active_idx' AND relnamespace = 'app'::regnamespace`,
	}
	for _, trigger := range []string{"tests_fill_owner", "questions_fill_owner", "question_groups_fill_owner", "media_assets_fill_owner", "classes_fill_teacher"} {
		gone["the trigger "+trigger] = `SELECT 1 FROM pg_trigger WHERE tgname = '` + trigger + `'`
	}
	for _, function := range []string{"fill_owner_from_created_by", "fill_owner_from_uploaded_by", "classes_fill_teacher"} {
		gone["the function "+function] = `SELECT 1 FROM pg_proc WHERE proname = '` + function + `' AND pronamespace = 'app'::regnamespace`
	}
	for name, query := range gone {
		if catalogHolds(t, conn, query) {
			t.Errorf("%s survived its contract migration", name)
		}
	}
}

func TestTheObjectsTheContractsKeptAreStillThere(t *testing.T) {
	conn := migrated(t)
	kept := map[string]string{
		"the index users_role_id_active_idx": `SELECT 1 FROM pg_class WHERE relname = 'users_role_id_active_idx' AND relnamespace = 'app'::regnamespace`,
		"the validated constraint users_role_id_not_null": `
			SELECT 1 FROM pg_constraint WHERE conname = 'users_role_id_not_null' AND conrelid = 'app.users'::regclass AND convalidated`,
		"the trigger users_last_admin":     `SELECT 1 FROM pg_trigger WHERE tgname = 'users_last_admin' AND tgenabled = 'O'`,
		"the trigger users_set_updated_at": `SELECT 1 FROM pg_trigger WHERE tgname = 'users_set_updated_at' AND tgenabled = 'O'`,
	}
	for _, key := range []string{"tests_owner_id_fkey", "questions_owner_id_fkey", "question_groups_owner_id_fkey", "media_assets_owner_id_fkey", "classes_teacher_id_fkey"} {
		kept["the validated RESTRICT foreign key "+key] = `
			SELECT 1 FROM pg_constraint WHERE conname = '` + key + `' AND contype = 'f' AND convalidated AND confdeltype = 'r'`
	}
	for name, query := range kept {
		if !catalogHolds(t, conn, query) {
			t.Errorf("%s is missing", name)
		}
	}
}

func TestNoTriggerIsLeftDisabled(t *testing.T) {
	conn := migrated(t)
	rows, err := conn.Query(`SELECT tgrelid::regclass::text || '.' || tgname FROM pg_trigger WHERE NOT tgisinternal AND tgenabled <> 'O'`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		t.Errorf("the trigger %s is not enabled", name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestAUserWithoutARoleIsRefused(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, _ fixture) {
		rejectsWith(t, tx, `null value in column "role_id" of relation "users"`,
			`INSERT INTO app.users (email, full_name) VALUES (gen_random_uuid()::text || '@example.com', 'Không vai trò')`)
	})
}

func TestAnAccountsCreatorIsOptionalAndSurvivesItsCreator(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, _ fixture) {
		const insert = `
			INSERT INTO app.users (email, full_name, role_id, created_by)
			VALUES (gen_random_uuid()::text || '@example.com', 'Tài khoản', (SELECT id FROM app.roles WHERE builtin_key = $1), $2)
			RETURNING id::text`
		var staff, selfJoined, made string
		if err := tx.QueryRow(insert, "teacher", nil).Scan(&staff); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRow(insert, "student", nil).Scan(&selfJoined); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRow(insert, "student", staff).Scan(&made); err != nil {
			t.Fatal(err)
		}
		creatorOf := func(id string) *string {
			t.Helper()
			var creator *string
			if err := tx.QueryRow(`SELECT created_by::text FROM app.users WHERE id = $1`, id).Scan(&creator); err != nil {
				t.Fatal(err)
			}
			return creator
		}
		if creator := creatorOf(selfJoined); creator != nil {
			t.Errorf("a self-joined account has creator %s, want none", *creator)
		}
		if creator := creatorOf(made); creator == nil || *creator != staff {
			t.Fatalf("the created account's creator is %v, want %s", creator, staff)
		}
		if _, err := tx.Exec(`DELETE FROM app.users WHERE id = $1`, staff); err != nil {
			t.Fatalf("deleting the creator: %v", err)
		}
		if creator := creatorOf(made); creator != nil {
			t.Errorf("the account still names its deleted creator %s", *creator)
		}
	})
}
