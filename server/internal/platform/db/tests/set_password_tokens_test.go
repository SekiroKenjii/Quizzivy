//go:build integration

package db_test

import (
	"database/sql"
	"strings"
	"testing"
	"time"
)

const insertLink = `
	INSERT INTO app.set_password_tokens (user_id, token_hash, purpose, created_at, expires_at)
	VALUES ($1::uuid, $2::bytea, $3, $4::timestamptz, $5::timestamptz)`

var someHash = []byte("0123456789abcdef0123456789abcdef")

func linkAt() time.Time { return time.Date(2026, 10, 14, 12, 0, 0, 123456000, time.UTC) }

func TestASetPasswordLinkStoresASha256SizedHashOnly(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "set_password_tokens_token_hash_check", insertLink,
			f.studentID, []byte("too-short"), "invite", linkAt(), linkAt().Add(time.Hour))
	})
}

func TestASetPasswordLinkIsAnInviteOrAReset(t *testing.T) {
	for _, purpose := range []string{"", "Invite", "forgot", "login"} {
		t.Run(purpose, func(t *testing.T) {
			withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
				rejectsWith(t, tx, "set_password_tokens_purpose_check", insertLink,
					f.studentID, someHash, purpose, linkAt(), linkAt().Add(time.Hour))
			})
		})
	}
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx, insertLink, f.studentID, someHash, "invite", linkAt(), linkAt().Add(time.Hour))
		mustExec(t, tx, insertLink, f.studentID, []byte("fedcba9876543210fedcba9876543210"), "reset", linkAt(), linkAt().Add(time.Hour))
	})
}

func TestTwoLinksNeverShareAHash(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx, insertLink, f.studentID, someHash, "invite", linkAt(), linkAt().Add(time.Hour))
		rejectsWith(t, tx, "set_password_tokens_token_hash_key", insertLink,
			f.adminID, someHash, "reset", linkAt(), linkAt().Add(time.Hour))
	})
}

func TestALinkLivesNoLongerThanTwentyFourHours(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx, insertLink, f.studentID, someHash, "invite", linkAt(), linkAt().Add(24*time.Hour))
		mustExec(t, tx, insertLink, f.studentID, []byte("fedcba9876543210fedcba9876543210"), "reset", linkAt(), linkAt().Add(time.Minute))
	})
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "set_password_tokens_lifetime_check", insertLink,
			f.studentID, someHash, "invite", linkAt(), linkAt().Add(24*time.Hour+time.Microsecond))
	})
}

func TestDeletingAnAccountDeletesItsLinks(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx, insertLink, f.studentID, someHash, "invite", linkAt(), linkAt().Add(time.Hour))

		mustExec(t, tx, `DELETE FROM app.users WHERE id = $1::uuid`, f.studentID)

		var left int
		if err := tx.QueryRow(`SELECT count(*) FROM app.set_password_tokens WHERE token_hash = $1::bytea`, someHash).Scan(&left); err != nil {
			t.Fatal(err)
		}
		if left != 0 {
			t.Errorf("%d links outlived their account", left)
		}
	})
}

func TestDeletingTheUserWhoIssuedALinkLeavesTheLinkWithNoIssuer(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		var issuer string
		if err := tx.QueryRow(`
			INSERT INTO app.users (email, full_name, role_id)
			VALUES ('issuer-' || gen_random_uuid() || '@example.com', 'Người cấp', (SELECT id FROM app.roles WHERE builtin_key = 'student'))
			RETURNING id::text`).Scan(&issuer); err != nil {
			t.Fatal(err)
		}
		mustExec(t, tx, `
			INSERT INTO app.set_password_tokens (user_id, token_hash, purpose, created_by, created_at, expires_at)
			VALUES ($1::uuid, $2::bytea, 'invite', $3::uuid, $4::timestamptz, $5::timestamptz)`,
			f.studentID, someHash, issuer, linkAt(), linkAt().Add(time.Hour))

		mustExec(t, tx, `DELETE FROM app.users WHERE id = $1::uuid`, issuer)

		var createdBy sql.NullString
		if err := tx.QueryRow(`SELECT created_by::text FROM app.set_password_tokens WHERE token_hash = $1::bytea`, someHash).Scan(&createdBy); err != nil {
			t.Fatalf("the link did not survive its issuer: %v", err)
		}
		if createdBy.Valid {
			t.Errorf("created_by = %s, want NULL", createdBy.String)
		}
	})
}

func TestTheLinksOfAnAccountAreFoundByAnIndexOnTheAccount(t *testing.T) {
	conn := migrated(t)
	var def string
	if err := conn.QueryRow(`SELECT indexdef FROM pg_indexes WHERE schemaname = 'app' AND indexname = 'set_password_tokens_user_idx'`).Scan(&def); err != nil {
		t.Fatalf("set_password_tokens_user_idx is missing: %v", err)
	}
	if want := "ON app.set_password_tokens USING btree (user_id)"; !strings.Contains(def, want) {
		t.Errorf("index = %s, want one %s, which is full so that the cascade from users uses it too", def, want)
	}
}

func TestTheAppRoleReadsAndWritesLinksAndOwnsNoTable(t *testing.T) {
	conn := migrated(t)
	for _, privilege := range []string{"SELECT", "INSERT", "UPDATE"} {
		var ok bool
		if err := conn.QueryRow(`SELECT has_table_privilege('quizzivy_app', 'app.set_password_tokens', $1)`, privilege).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Errorf("quizzivy_app lacks %s on app.set_password_tokens", privilege)
		}
	}
	var owner string
	if err := conn.QueryRow(`SELECT tableowner FROM pg_tables WHERE schemaname = 'app' AND tablename = 'set_password_tokens'`).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	if owner == "quizzivy_app" {
		t.Error("quizzivy_app owns the table; it owns nothing")
	}
}
