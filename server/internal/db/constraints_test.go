package db_test

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"quizzivy/internal/db"
)

// Every constraint in 20-data-model.md exists because a bug would otherwise be
// possible and silent. These assert that each one actually fires -- a CHECK
// nobody tests is a comment with extra syntax.
//
// Each case runs in a transaction that is always rolled back, so the tests are
// order-independent and leave nothing behind.

func migrated(t *testing.T) *sql.DB {
	t.Helper()
	conn := openMigrate(t)
	if err := goose.Up(conn, db.MigrationsDir(t)); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return conn
}

// fixture holds ids created fresh for one test.
//
// Nothing here hard-codes an id. An earlier version did, and every test in this
// file broke the moment `make seed` ran -- the seed happened to use one of the
// same uuids. A test suite that assumes an empty database is a suite that only
// passes on a machine nobody has used.
type fixture struct {
	adminID   string
	studentID string
	classID   string
}

// withTx runs fn inside a transaction that is always rolled back, having first
// created an admin, a student and a class with unique identities.
func withTx(t *testing.T, conn *sql.DB, fn func(tx *sql.Tx, f fixture)) {
	t.Helper()
	tx, err := conn.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatalf("rand: %v", err)
	}
	tag := hex.EncodeToString(nonce)

	var f fixture
	must := func(dest *string, query string, args ...any) {
		t.Helper()
		if err := tx.QueryRow(query, args...).Scan(dest); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	must(&f.adminID,
		`INSERT INTO app.users (email, full_name, role, password_hash)
		 VALUES ($1, 'Thuong', 'admin', 'hash') RETURNING id`, "admin-"+tag+"@example.com")
	// role defaults to 'student' (§13.3), so it is omitted rather than restated.
	must(&f.studentID,
		`INSERT INTO app.users (email, full_name) VALUES ($1, 'Học viên') RETURNING id`,
		"student-"+tag+"@example.com")
	must(&f.classID,
		`INSERT INTO app.classes (name) VALUES ($1) RETURNING id`, "Lớp "+tag)

	fn(tx, f)
}

// rejectsWith asserts that the statement fails, and that it fails for the
// stated reason rather than by accident.
func rejectsWith(t *testing.T, tx *sql.Tx, wantConstraint, stmt string, args ...any) {
	t.Helper()
	_, err := tx.Exec(stmt, args...)
	if err == nil {
		t.Fatalf("statement was accepted but should violate %s:\n  %.120s", wantConstraint, stmt)
	}
	if !strings.Contains(err.Error(), wantConstraint) {
		t.Errorf("rejected for the wrong reason; wanted %s, got: %v", wantConstraint, err)
	}
}

// ---------------------------------------------------------------- users

func TestMustChangePasswordRequiresAPassword(t *testing.T) {
	// [D-16] §5.4: a Google-only account flagged mustChangePassword would be
	// trapped on a page it cannot complete, since the redirect is global.
	withTx(t, migrated(t), func(tx *sql.Tx, _ fixture) {
		rejectsWith(t, tx, "users_must_change_needs_password",
			`INSERT INTO app.users (email, full_name, must_change_password)
			 VALUES ('google-only@example.com', 'Học viên', true)`)
	})
}

func TestMustChangePasswordIsFineWithAPassword(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, _ fixture) {
		mustExec(t, tx,
			`INSERT INTO app.users (email, full_name, password_hash, must_change_password)
			 VALUES ('has-password@example.com', 'Học viên', 'argon2id$...', true)`)
	})
}

func TestEmailUniquenessIgnoresCase(t *testing.T) {
	// A plain UNIQUE (email) would let these coexist, which breaks §5.1's rule
	// that a verified Google email links to "the" existing user -- with two
	// candidates, §5.3's resolution order has no defined answer.
	withTx(t, migrated(t), func(tx *sql.Tx, _ fixture) {
		mustExec(t, tx, `INSERT INTO app.users (email, full_name) VALUES ('Hoc.Vien@Example.com', 'A')`)
		rejectsWith(t, tx, "users_email_lower_key",
			`INSERT INTO app.users (email, full_name) VALUES ('hoc.vien@example.com', 'B')`)
	})
}

// ------------------------------------------------------ user_identities

func TestOneGoogleIdentityPerUser(t *testing.T) {
	// [D-08] Without this, "unlink Google" (§15) has no defined meaning.
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.user_identities (user_id, provider, provider_user_id, email_at_link)
			 VALUES ($1, 'google', 'sub-1', 'a@example.com')`, f.adminID)
		rejectsWith(t, tx, "user_identities_user_id_provider_key",
			`INSERT INTO app.user_identities (user_id, provider, provider_user_id, email_at_link)
			 VALUES ($1, 'google', 'sub-2', 'b@example.com')`, f.adminID)
	})
}

func TestOneGoogleAccountCannotReachTwoUsers(t *testing.T) {
	// The account-takeover shape §5.1 guards against, at the storage layer.
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.user_identities (user_id, provider, provider_user_id, email_at_link)
			 VALUES ($1, 'google', 'shared-sub', 'a@example.com')`, f.adminID)
		rejectsWith(t, tx, "user_identities_provider_provider_user_id_key",
			`INSERT INTO app.user_identities (user_id, provider, provider_user_id, email_at_link)
			 VALUES ($1, 'google', 'shared-sub', 'b@example.com')`, f.studentID)
	})
}

// ------------------------------------------------------- refresh_tokens

func TestRefreshTokenHashMustBeSha256Sized(t *testing.T) {
	// A wrong-length hash would otherwise surface as a token that can never
	// match -- an unexplained logout rather than an error.
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "refresh_tokens_token_hash_check",
			`INSERT INTO app.refresh_tokens (user_id, family_id, token_hash, expires_at)
			 VALUES ($1, gen_random_uuid(), '\x0102'::bytea, now() + interval '30 days')`, f.adminID)
	})
}

func TestRefreshTokenCannotExpireBeforeItIsIssued(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "refresh_tokens_check",
			`INSERT INTO app.refresh_tokens (user_id, family_id, token_hash, issued_at, expires_at)
			 VALUES ($1, gen_random_uuid(), sha256('x'::bytea), now(), now() - interval '1 hour')`, f.adminID)
	})
}

// ----------------------------------------------------- class_join_codes

func TestJoinCodeCannotExceedMaxUses(t *testing.T) {
	// [D-09] §6.5 treats the code as a bearer secret. Enforcing exhaustion only
	// in the handler leaves an over-used code possible and silent.
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "class_join_codes_not_over_used",
			`INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, max_uses, uses_count, created_by)
			 VALUES ($1, sha256('c'::bytea), 'AB12', now() + interval '30 days', 5, 6, $2)`,
			f.classID, f.adminID)
	})
}

func TestJoinCodeCannotExpireBeforeItIsCreated(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "class_join_codes_expiry_after_creation",
			`INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, created_by)
			 VALUES ($1, sha256('c'::bytea), 'AB12', now() - interval '1 day', $2)`,
			f.classID, f.adminID)
	})
}

func TestOnlyOneActiveCodePerClass(t *testing.T) {
	// §6.1. Rotation must revoke before inserting, in one transaction; this is
	// what makes "must" true rather than advisory.
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, created_by)
			 VALUES ($1, sha256('one'::bytea), 'AB12', now() + interval '30 days', $2)`,
			f.classID, f.adminID)
		rejectsWith(t, tx, "class_join_codes_one_active",
			`INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, created_by)
			 VALUES ($1, sha256('two'::bytea), 'CD34', now() + interval '30 days', $2)`,
			f.classID, f.adminID)
	})
}

func TestRevokingFreesTheActiveSlot(t *testing.T) {
	// The partial index keys on revoked_at IS NULL, so a revoked code must not
	// block a replacement -- otherwise rotation could never succeed at all.
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, revoked_at, created_by)
			 VALUES ($1, sha256('old'::bytea), 'AB12', now() + interval '30 days', now(), $2)`,
			f.classID, f.adminID)
		mustExec(t, tx,
			`INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, created_by)
			 VALUES ($1, sha256('new'::bytea), 'CD34', now() + interval '30 days', $2)`,
			f.classID, f.adminID)
	})
}

func TestAnExpiredCodeStillOccupiesTheActiveSlot(t *testing.T) {
	// Deliberate, and easy to misread as a bug: the partial index keys on
	// revoked_at, not on expiry. §6.1 says only rotation revokes, so an expired
	// code still holds the slot until it is explicitly revoked.
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, created_by)
			 VALUES ($1, sha256('expired'::bytea), 'AB12', now() + interval '1 second', $2)`,
			f.classID, f.adminID)
		rejectsWith(t, tx, "class_join_codes_one_active",
			`INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, created_by)
			 VALUES ($1, sha256('fresh'::bytea), 'CD34', now() + interval '30 days', $2)`,
			f.classID, f.adminID)
	})
}

// -------------------------------------------------------- class_members

func TestJoinSourceMustMatchTheCodeReference(t *testing.T) {
	// [D-10] joined_via = 'admin' iff join_code_id IS NULL. Without it the
	// member list §6.4 relies on could claim a code-based join with no code.
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "class_members_source_consistent",
			`INSERT INTO app.class_members (class_id, user_id, joined_via)
			 VALUES ($1, $2, 'join_code')`, f.classID, f.studentID)
	})
}

func TestAdminJoinNeedsNoCode(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.class_members (class_id, user_id, joined_via, added_by)
			 VALUES ($1, $2, 'admin', $3)`, f.classID, f.studentID, f.adminID)
	})
}

func TestCodeJoinRecordsWhichCode(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		var codeID string
		if err := tx.QueryRow(
			`INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, created_by)
			 VALUES ($1, sha256('c'::bytea), 'AB12', now() + interval '30 days', $2) RETURNING id`,
			f.classID, f.adminID).Scan(&codeID); err != nil {
			t.Fatal(err)
		}
		mustExec(t, tx,
			`INSERT INTO app.class_members (class_id, user_id, joined_via, join_code_id)
			 VALUES ($1, $2, 'join_code', $3)`, f.classID, f.studentID, codeID)
	})
}

// ------------------------------------------------------------ audit_log

func TestAuditRowsSurviveTheirActor(t *testing.T) {
	// §13.4. An audit trail that disappears when the actor is deleted is not an
	// audit trail.
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.audit_log (actor_user_id, action, entity, entity_id)
			 VALUES ($1, 'class.enrol', 'class', $2)`, f.studentID, f.classID)
		mustExec(t, tx, `DELETE FROM app.users WHERE id = $1`, f.studentID)

		var rows int
		var actor sql.NullString
		if err := tx.QueryRow(
			`SELECT count(*), max(actor_user_id::text) FROM app.audit_log WHERE entity_id = $1`,
			f.classID).Scan(&rows, &actor); err != nil {
			t.Fatal(err)
		}
		if rows != 1 {
			t.Errorf("audit rows = %d after deleting the actor, want 1", rows)
		}
		if actor.Valid {
			t.Error("actor_user_id should be NULL after the user is deleted, not dangling")
		}
	})
}

func TestDeletingAUserWhoIssuedACodeIsRefused(t *testing.T) {
	// created_by is ON DELETE RESTRICT: the record of who issued a bearer
	// secret must not be erasable by deleting the user.
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, created_by)
			 VALUES ($1, sha256('c'::bytea), 'AB12', now() + interval '30 days', $2)`,
			f.classID, f.adminID)
		rejectsWith(t, tx, "class_join_codes_created_by_fkey",
			`DELETE FROM app.users WHERE id = $1`, f.adminID)
	})
}
