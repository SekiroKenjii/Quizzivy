//go:build integration

package db_test

import (
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"quizzivy/internal/platform/db"
)

const (
	addUsersActivitySuffix   = "_add_users_activity.sql"
	createActivityDaysSuffix = "_create_user_activity_days.sql"
)

func TestTheActivityColumnsStartFromTheNewestSignOfLife(t *testing.T) {
	dsn := expandScratch(t)
	migrate := openAs(t, dsn)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	goose.SetLogger(goose.NopLogger())
	dir := db.MigrationsDir(t)
	if err := goose.UpTo(migrate, dir, versionBefore(t, dir, addUsersActivitySuffix)); err != nil {
		t.Fatalf("up to v0.11.7: %v", err)
	}

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	user := func(email, role string) string {
		t.Helper()
		var id string
		must(migrate.QueryRow(`
			INSERT INTO app.users (email, full_name, role_id)
			VALUES ($1, $1, (SELECT id FROM app.roles WHERE builtin_key = $2)) RETURNING id::text`, email, role).Scan(&id))
		return id
	}
	ago := func(d time.Duration) time.Time { return time.Now().Add(-d).Truncate(time.Microsecond) }

	teacher := user("teacher@example.com", "admin")
	byAudit := user("by-audit@example.com", "student")
	byToken := user("by-token@example.com", "student")
	byAttempt := user("by-attempt@example.com", "student")
	never := user("never@example.com", "student")

	token := func(id string, at time.Time) {
		t.Helper()
		_, err := migrate.Exec(`
			INSERT INTO app.refresh_tokens (user_id, family_id, token_hash, issued_at, expires_at)
			VALUES ($1, gen_random_uuid(), sha256(gen_random_uuid()::text::bytea), $2::timestamptz, $2::timestamptz + interval '30 days')`, id, at)
		must(err)
	}
	audit := func(id string, at time.Time) {
		t.Helper()
		_, err := migrate.Exec(`
			INSERT INTO app.audit_log (actor_user_id, action, entity, entity_id, occurred_at)
			VALUES ($1, 'user.profile_updated', 'user', $1, $2::timestamptz)`, id, at)
		must(err)
	}
	token(byAudit, ago(40*24*time.Hour))
	audit(byAudit, ago(2*24*time.Hour))
	token(byToken, ago(9*24*time.Hour))
	token(byToken, ago(3*24*time.Hour))

	var testID, versionID, assignmentID string
	must(migrate.QueryRow(`
		INSERT INTO app.tests (title, status, current_version, created_by, owner_id)
		VALUES ('T', 'published', 1, $1::uuid, $1::uuid) RETURNING id::text`, teacher).Scan(&testID))
	must(migrate.QueryRow(`
		INSERT INTO app.test_versions (test_id, version, total_points, published_by)
		VALUES ($1::uuid, 1, 10, $2::uuid) RETURNING id::text`, testID, teacher).Scan(&versionID))
	must(migrate.QueryRow(`
		INSERT INTO app.assignments (test_id, test_version_id, opens_at, closes_at, duration_minutes, created_by, published_at)
		VALUES ($1::uuid, $2::uuid, now() - interval '30 days', now() + interval '30 days', 45, $3::uuid, now())
		RETURNING id::text`, testID, versionID, teacher).Scan(&assignmentID))
	attemptAt := ago(5 * 24 * time.Hour)
	_, err := migrate.Exec(`
		INSERT INTO app.attempts (assignment_id, test_version_id, student_id, attempt_no, status,
		       session_id, shuffle_seed, beacon_token_hash, started_at, deadline_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 1, 'in_progress', gen_random_uuid(), 1, sha256('b'::bytea), $4::timestamptz, $4::timestamptz + interval '1 hour')`,
		assignmentID, versionID, byAttempt, attemptAt)
	must(err)
	var updatedBefore time.Time
	must(migrate.QueryRow(`SELECT updated_at FROM app.users WHERE id = $1`, byAudit).Scan(&updatedBefore))

	if err := goose.UpTo(migrate, dir, versionBefore(t, dir, createActivityDaysSuffix)); err != nil {
		t.Fatalf("up through add_users_activity: %v", err)
	}

	read := func(id string) (signedIn, active *time.Time, method *string, changed *time.Time, updated time.Time) {
		t.Helper()
		must(migrate.QueryRow(`
			SELECT last_sign_in_at, last_active_at, last_sign_in_method, password_changed_at, updated_at
			  FROM app.users WHERE id = $1`, id).Scan(&signedIn, &active, &method, &changed, &updated))
		return
	}
	same := func(got *time.Time, want time.Time) bool { return got != nil && got.Equal(want) }

	var auditAt time.Time
	must(migrate.QueryRow(`SELECT max(occurred_at) FROM app.audit_log WHERE actor_user_id = $1`, byAudit).Scan(&auditAt))
	var tokenAt time.Time
	must(migrate.QueryRow(`SELECT max(issued_at) FROM app.refresh_tokens WHERE user_id = $1`, byToken).Scan(&tokenAt))

	for name, c := range map[string]struct {
		id   string
		want time.Time
	}{
		"the audit entry beats an older token": {byAudit, auditAt},
		"the newest of two tokens":             {byToken, tokenAt},
		"an attempt alone":                     {byAttempt, attemptAt},
	} {
		signedIn, active, method, changed, _ := read(c.id)
		if !same(signedIn, c.want) || !same(active, c.want) {
			t.Errorf("%s: last_sign_in_at %v and last_active_at %v, want both %v", name, signedIn, active, c.want)
		}
		if method != nil || changed != nil {
			t.Errorf("%s: the backfill invented a method %v or a password change %v", name, method, changed)
		}
	}
	signedIn, active, _, _, _ := read(never)
	if signedIn != nil || active != nil {
		t.Errorf("an account with no trace reads %v and %v, want Not signed in", signedIn, active)
	}
	if _, _, _, _, updated := read(byAudit); !updated.Equal(updatedBefore) {
		t.Errorf("the backfill moved updated_at from %v to %v", updatedBefore, updated)
	}
}

func TestAnInsertWithTheV0100ColumnListStillSucceeds(t *testing.T) {
	migrated(t)
	app := openAs(t, appRoleDSN(t, db.TestDSN(t)))
	tx, err := app.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	var signedIn *time.Time
	if err := tx.QueryRow(`
		INSERT INTO app.users (email, full_name, role_id, password_hash, must_change_password, display_name, phone, avatar_key, locale, time_zone, preferences, created_by)
		VALUES ('column-list@example.com', 'Cột', (SELECT id FROM app.roles WHERE builtin_key = 'student'), 'x', true, NULL, NULL, NULL, NULL, NULL, '{}'::jsonb, NULL)
		RETURNING last_sign_in_at`).Scan(&signedIn); err != nil {
		t.Fatalf("the v0.10.0 column list was refused: %v", err)
	}
	if signedIn != nil {
		t.Errorf("a new account reads signed in at %v", signedIn)
	}
	if _, err := tx.Exec(`UPDATE app.users SET last_sign_in_method = 'ldap' WHERE email = 'column-list@example.com'`); err == nil {
		t.Error("last_sign_in_method accepted 'ldap'")
	}
}
