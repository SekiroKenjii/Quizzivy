//go:build integration

package db_test

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"quizzivy/internal/platform/db"
)

const requireEncryptedJoinCodesSuffix = "_require_encrypted_join_codes.sql"

func TestRequireEncryptedJoinCodesRefusesALiveLegacyCodeAndRevokesAnExpiredOne(t *testing.T) {
	dsn := expandScratch(t)
	migrate := openAs(t, dsn)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	goose.SetLogger(goose.NopLogger())
	dir := db.MigrationsDir(t)
	before := versionBefore(t, dir, requireEncryptedJoinCodesSuffix)
	if err := goose.UpTo(migrate, dir, before); err != nil {
		t.Fatalf("up to %d: %v", before, err)
	}

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var teacher string
	must(migrate.QueryRow(`
		INSERT INTO app.users (email, full_name, role_id)
		VALUES ('codes@example.com', 'Codes', (SELECT id FROM app.roles WHERE builtin_key = 'admin')) RETURNING id::text`).Scan(&teacher))
	code := func(name, expires, revoked string, sealed bool) string {
		t.Helper()
		var class, id string
		must(migrate.QueryRow(`INSERT INTO app.classes (name, teacher_id) VALUES ($1, $2::uuid) RETURNING id::text`, name, teacher).Scan(&class))
		insert := `
			INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, created_at, expires_at, revoked_at, created_by)
			VALUES ($1::uuid, sha256($2::bytea), 'AB12', now() - interval '60 days', now() + $3::interval, now() + $4::interval, $5::uuid)
			RETURNING id::text`
		if sealed {
			insert = `
			INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, created_at, expires_at, revoked_at, created_by, lookup_scheme, key_id, code_ciphertext)
			VALUES ($1::uuid, sha256($2::bytea), 'AB12', now() - interval '60 days', now() + $3::interval, now() + $4::interval, $5::uuid, 2, 7, decode(repeat('00', 36), 'hex'))
			RETURNING id::text`
		}
		var revokedAt any
		if revoked != "" {
			revokedAt = revoked
		}
		must(migrate.QueryRow(insert, class, name, expires, revokedAt, teacher).Scan(&id))
		return id
	}
	read := func(id string) (expires time.Time, revoked *time.Time) {
		t.Helper()
		must(migrate.QueryRow(`SELECT expires_at, revoked_at FROM app.class_join_codes WHERE id = $1::uuid`, id).Scan(&expires, &revoked))
		return expires, revoked
	}
	constraint := func() (exists, valid bool) {
		t.Helper()
		err := migrate.QueryRow(`
			SELECT convalidated FROM pg_constraint
			 WHERE conrelid = 'app.class_join_codes'::regclass AND conname = 'class_join_codes_live_encrypted'`).Scan(&valid)
		if err == sql.ErrNoRows {
			return false, false
		}
		must(err)
		return true, valid
	}

	live := code("live legacy", "10 days", "", false)
	expired := code("expired legacy", "-1 day", "", false)
	revoked := code("revoked legacy", "10 days", "-5 days", false)
	sealed := code("sealed", "10 days", "", true)
	_, revokedAt := read(revoked)

	err := goose.UpTo(migrate, dir, before+1)
	if err == nil || !strings.Contains(err.Error(), "holds 1 live join code(s) without a ciphertext") {
		t.Fatalf("up over a live legacy code answered %v, want the refusal naming one code", err)
	}
	if exists, _ := constraint(); exists {
		t.Error("the refused migration left class_join_codes_live_encrypted behind")
	}
	if _, r := read(expired); r != nil {
		t.Errorf("the refused migration revoked the expired code at %v", r)
	}

	must(run(migrate, `UPDATE app.class_join_codes SET revoked_at = now() WHERE id = $1::uuid`, live))
	must(goose.UpTo(migrate, dir, before+1))
	if exists, valid := constraint(); !exists || !valid {
		t.Fatalf("class_join_codes_live_encrypted exists %v, validated %v; want both", exists, valid)
	}
	if expires, r := read(expired); r == nil || !r.Equal(expires) {
		t.Errorf("the expired legacy code reads revoked at %v, want its expiry %v", r, expires)
	}
	if _, r := read(revoked); r == nil || !r.Equal(*revokedAt) {
		t.Errorf("the revoked legacy code reads revoked at %v, want %v unchanged", r, revokedAt)
	}
	if _, r := read(sealed); r != nil {
		t.Errorf("the sealed code was revoked at %v", r)
	}
	if err := run(migrate, `UPDATE app.class_join_codes SET revoked_at = NULL WHERE id = $1::uuid`, expired); err == nil {
		t.Error("an unrevoked legacy code was accepted under the constraint")
	}

	must(goose.Down(migrate, dir))
	if exists, _ := constraint(); exists {
		t.Error("Down left class_join_codes_live_encrypted behind")
	}
	if _, r := read(expired); r == nil {
		t.Error("Down undid the revocation of the expired legacy code")
	}
	must(run(migrate, `UPDATE app.class_join_codes SET revoked_at = NULL WHERE id = $1::uuid`, expired))
}

func run(conn *sql.DB, query string, args ...any) error {
	_, err := conn.Exec(query, args...)
	return err
}
