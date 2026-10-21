package db_test

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"quizzivy/internal/platform/db"
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
	hasColumn := func(table, column string) bool {
		t.Helper()
		var exists bool
		if err := tx.QueryRow(`SELECT EXISTS (SELECT 1 FROM pg_attribute
			WHERE attrelid = ('app.' || $1)::regclass AND attname = $2 AND NOT attisdropped)`, table, column).Scan(&exists); err != nil {
			t.Fatalf("fixture: %v", err)
		}
		return exists
	}
	if hasColumn("users", "role_id") {
		must(&f.adminID,
			`INSERT INTO app.users (email, full_name, role_id, password_hash)
			 VALUES ($1, 'Thuong', (SELECT id FROM app.roles WHERE builtin_key = 'admin'), 'hash') RETURNING id`, "admin-"+tag+"@example.com")
		must(&f.studentID,
			`INSERT INTO app.users (email, full_name, role_id)
			 VALUES ($1, 'Học viên', (SELECT id FROM app.roles WHERE builtin_key = 'student')) RETURNING id`,
			"student-"+tag+"@example.com")
	} else {
		must(&f.adminID,
			`INSERT INTO app.users (email, full_name, role, password_hash)
			 VALUES ($1, 'Thuong', 'admin', 'hash') RETURNING id`, "admin-"+tag+"@example.com")
		must(&f.studentID,
			`INSERT INTO app.users (email, full_name) VALUES ($1, 'Học viên') RETURNING id`,
			"student-"+tag+"@example.com")
	}
	if hasColumn("classes", "teacher_id") {
		must(&f.classID,
			`INSERT INTO app.classes (name, teacher_id) VALUES ($1, $2) RETURNING id`, "Lớp "+tag, f.adminID)
	} else {
		must(&f.classID,
			`INSERT INTO app.classes (name) VALUES ($1) RETURNING id`, "Lớp "+tag)
	}

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
	withTx(t, migrated(t), func(tx *sql.Tx, _ fixture) {
		rejectsWith(t, tx, "users_must_change_needs_password",
			`INSERT INTO app.users (email, full_name, must_change_password, role_id)
			 VALUES ('google-only@example.com', 'Học viên', true, (SELECT id FROM app.roles WHERE builtin_key = 'student'))`)
	})
}

func TestMustChangePasswordIsFineWithAPassword(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, _ fixture) {
		mustExec(t, tx,
			`INSERT INTO app.users (email, full_name, password_hash, must_change_password, role_id)
			 VALUES ('has-password@example.com', 'Học viên', 'argon2id$...', true, (SELECT id FROM app.roles WHERE builtin_key = 'student'))`)
	})
}

func TestEmailUniquenessIgnoresCase(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, _ fixture) {
		mustExec(t, tx, `INSERT INTO app.users (email, full_name, role_id) VALUES ('Hoc.Vien@Example.com', 'A', (SELECT id FROM app.roles WHERE builtin_key = 'student'))`)
		rejectsWith(t, tx, "users_email_lower_key",
			`INSERT INTO app.users (email, full_name, role_id) VALUES ('hoc.vien@example.com', 'B', (SELECT id FROM app.roles WHERE builtin_key = 'student'))`)
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
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "class_join_codes_not_over_used",
			`INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, max_uses, uses_count, created_by, lookup_scheme, key_id, code_ciphertext)
			 VALUES ($1, sha256('c'::bytea), 'AB12', now() + interval '30 days', 5, 6, $2, 2, 7, decode(repeat('00', 36), 'hex'))`,
			f.classID, f.adminID)
	})
}

func TestJoinCodeCannotExpireBeforeItIsCreated(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "class_join_codes_expiry_after_creation",
			`INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, created_by, lookup_scheme, key_id, code_ciphertext)
			 VALUES ($1, sha256('c'::bytea), 'AB12', now() - interval '1 day', $2, 2, 7, decode(repeat('00', 36), 'hex'))`,
			f.classID, f.adminID)
	})
}

func TestOnlyOneActiveCodePerClass(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, created_by, lookup_scheme, key_id, code_ciphertext)
			 VALUES ($1, sha256('one'::bytea), 'AB12', now() + interval '30 days', $2, 2, 7, decode(repeat('00', 36), 'hex'))`,
			f.classID, f.adminID)
		rejectsWith(t, tx, "class_join_codes_one_active",
			`INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, created_by, lookup_scheme, key_id, code_ciphertext)
			 VALUES ($1, sha256('two'::bytea), 'CD34', now() + interval '30 days', $2, 2, 7, decode(repeat('00', 36), 'hex'))`,
			f.classID, f.adminID)
	})
}

func TestRevokingFreesTheActiveSlot(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, revoked_at, created_by, lookup_scheme, key_id, code_ciphertext)
			 VALUES ($1, sha256('old'::bytea), 'AB12', now() + interval '30 days', now(), $2, 2, 7, decode(repeat('00', 36), 'hex'))`,
			f.classID, f.adminID)
		mustExec(t, tx,
			`INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, created_by, lookup_scheme, key_id, code_ciphertext)
			 VALUES ($1, sha256('new'::bytea), 'CD34', now() + interval '30 days', $2, 2, 7, decode(repeat('00', 36), 'hex'))`,
			f.classID, f.adminID)
	})
}

func TestAnExpiredCodeStillOccupiesTheActiveSlot(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, created_by, lookup_scheme, key_id, code_ciphertext)
			 VALUES ($1, sha256('expired'::bytea), 'AB12', now() + interval '1 second', $2, 2, 7, decode(repeat('00', 36), 'hex'))`,
			f.classID, f.adminID)
		rejectsWith(t, tx, "class_join_codes_one_active",
			`INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, created_by, lookup_scheme, key_id, code_ciphertext)
			 VALUES ($1, sha256('fresh'::bytea), 'CD34', now() + interval '30 days', $2, 2, 7, decode(repeat('00', 36), 'hex'))`,
			f.classID, f.adminID)
	})
}

func TestAJoinCodeRowIsEitherLegacyOrFullySealed(t *testing.T) {
	const insert = `INSERT INTO app.class_join_codes
		       (class_id, code_hash, code_hint, expires_at, created_by, lookup_scheme, code_ciphertext, key_id, revoked_at)
		VALUES ($1, sha256(gen_random_uuid()::text::bytea), 'AB12', now() + interval '30 days', $2, $3, $4, $5, $6)`
	sealed := make([]byte, 36)
	for name, c := range map[string]struct {
		constraint string
		scheme     int
		ciphertext []byte
		keyID      any
	}{
		"a keyed row without its ciphertext":  {"class_join_codes_scheme_consistent", 2, nil, 7},
		"a keyed row without its key id":      {"class_join_codes_scheme_consistent", 2, sealed, nil},
		"a legacy row with a ciphertext":      {"class_join_codes_scheme_consistent", 1, sealed, 7},
		"a legacy row with only a ciphertext": {"class_join_codes_scheme_consistent", 1, sealed, nil},
		"a legacy row with only a key id":     {"class_join_codes_scheme_consistent", 1, nil, 7},
		"a ciphertext of the wrong length":    {"class_join_codes_ciphertext_length", 2, make([]byte, 35), 7},
		"key id zero":                         {"class_join_codes_key_id_nonzero", 2, sealed, 0},
		"an unknown scheme":                   {"class_join_codes_lookup_scheme_known", 3, nil, nil},
	} {
		t.Run(name, func(t *testing.T) {
			withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
				rejectsWith(t, tx, c.constraint, insert, f.classID, f.adminID, c.scheme, c.ciphertext, c.keyID, time.Now())
			})
		})
	}
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx, insert, f.classID, f.adminID, 2, sealed, 7, nil)
		mustExec(t, tx, `UPDATE app.class_join_codes SET revoked_at = now() WHERE class_id = $1`, f.classID)
		mustExec(t, tx, insert, f.classID, f.adminID, 1, nil, nil, time.Now())
	})
}

func TestACodeThatIsNotRevokedIsSealed(t *testing.T) {
	const insert = `INSERT INTO app.class_join_codes
		       (class_id, code_hash, code_hint, expires_at, created_by)
		VALUES ($1, sha256(gen_random_uuid()::text::bytea), 'AB12', $3::timestamptz, $2)`
	for name, expires := range map[string]time.Time{
		"a live legacy code":      time.Now().Add(30 * 24 * time.Hour),
		"an expiring legacy code": time.Now().Add(time.Second),
	} {
		t.Run(name, func(t *testing.T) {
			withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
				rejectsWith(t, tx, "class_join_codes_live_encrypted", insert, f.classID, f.adminID, expires)
			})
		})
	}
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx, `INSERT INTO app.class_join_codes
		       (class_id, code_hash, code_hint, expires_at, created_by, lookup_scheme, key_id, code_ciphertext)
		VALUES ($1, sha256(gen_random_uuid()::text::bytea), 'AB12', now() + interval '30 days', $2, 2, 7, decode(repeat('00', 36), 'hex'))`, f.classID, f.adminID)
		rejectsWith(t, tx, "class_join_codes_live_encrypted",
			`UPDATE app.class_join_codes SET lookup_scheme = 1, key_id = NULL, code_ciphertext = NULL WHERE class_id = $1`, f.classID)
	})
}

// -------------------------------------------------------- class_members

func TestJoinSourceMustMatchTheCodeReference(t *testing.T) {
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
			`INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, created_by, lookup_scheme, key_id, code_ciphertext)
			 VALUES ($1, sha256('c'::bytea), 'AB12', now() + interval '30 days', $2, 2, 7, decode(repeat('00', 36), 'hex')) RETURNING id`,
			f.classID, f.adminID,
		).Scan(&codeID); err != nil {
			t.Fatal(err)
		}
		mustExec(t, tx,
			`INSERT INTO app.class_members (class_id, user_id, joined_via, join_code_id)
			 VALUES ($1, $2, 'join_code', $3)`, f.classID, f.studentID, codeID)
	})
}

// ------------------------------------------------------------ audit_log

func TestAuditRowsSurviveTheirActor(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.audit_log (actor_user_id, action, entity, entity_id)
			 VALUES ($1, 'class.enrol', 'class', $2)`, f.studentID, f.classID)
		mustExec(t, tx, `DELETE FROM app.users WHERE id = $1`, f.studentID)

		var rows int
		var actor sql.NullString
		if err := tx.QueryRow(
			`SELECT count(*), max(actor_user_id::text) FROM app.audit_log WHERE entity_id = $1`,
			f.classID,
		).Scan(&rows, &actor); err != nil {
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
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, created_by, lookup_scheme, key_id, code_ciphertext)
			 VALUES ($1, sha256('c'::bytea), 'AB12', now() + interval '30 days', $2, 2, 7, decode(repeat('00', 36), 'hex'))`,
			f.classID, f.adminID)
		rejectsWith(t, tx, "class_join_codes_created_by_fkey",
			`DELETE FROM app.users WHERE id = $1`, f.adminID)
	})
}

// ── media_assets (§11.1) ──────────────────────────────────────────────────
//
// §11.1's limits are CHECKs rather than handler code alone. The handler
// validates first and says so in Vietnamese; these are what keep "we validate
// server-side" true when a later code path forgets.

func TestAnAudioAssetMustCarryItsDuration(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "media_assets_audio_has_duration",
			`INSERT INTO app.media_assets
			   (kind, storage_key, mime_type, bytes, original_filename, checksum_sha256, uploaded_by, owner_id)
			 VALUES ('audio', 'media/a.mp3', 'audio/mpeg', 1024, 'a.mp3', repeat('x',32)::bytea, $1, $1)`,
			f.adminID)
	})
}

func TestANonAudioAssetMustNotCarryADuration(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "media_assets_audio_has_duration",
			`INSERT INTO app.media_assets
			   (kind, storage_key, mime_type, bytes, duration_ms, original_filename, checksum_sha256, uploaded_by, owner_id)
			 VALUES ('image', 'media/i.png', 'image/png', 1024, 5000, 'i.png', repeat('x',32)::bytea, $1, $1)`,
			f.adminID)
	})
}

func TestAnOversizedAssetIsRejected(t *testing.T) {
	conn := migrated(t)
	withTx(t, conn, func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "media_assets_bytes_by_kind",
			`INSERT INTO app.media_assets
			   (kind, storage_key, mime_type, bytes, duration_ms, original_filename, checksum_sha256, uploaded_by, owner_id)
			 VALUES ('audio', 'media/big.mp3', 'audio/mpeg', 52428801, 1000, 'big.mp3', repeat('x',32)::bytea, $1, $1)`,
			f.adminID)
	})
	withTx(t, conn, func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "media_assets_bytes_by_kind",
			`INSERT INTO app.media_assets
			   (kind, storage_key, mime_type, bytes, original_filename, checksum_sha256, uploaded_by, owner_id)
			 VALUES ('image', 'media/big.png', 'image/png', 10485761, 'big.png', repeat('x',32)::bytea, $1, $1)`,
			f.adminID)
	})
}

func TestAnAssetAtTheLimitOfItsKindIsAccepted(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.media_assets
			   (kind, storage_key, mime_type, bytes, duration_ms, original_filename, checksum_sha256, uploaded_by, owner_id)
			 VALUES ('audio', 'media/limit.mp3', 'audio/mpeg', 52428800, 1000, 'limit.mp3', repeat('x',32)::bytea, $1, $1)`,
			f.adminID)
		mustExec(t, tx,
			`INSERT INTO app.media_assets
			   (kind, storage_key, mime_type, bytes, duration_ms, original_filename, checksum_sha256, uploaded_by, owner_id)
			 VALUES ('audio', 'media/over-the-image-limit.mp3', 'audio/mpeg', 10485761, 1000, 'eleven.mp3', repeat('x',32)::bytea, $1, $1)`,
			f.adminID)
		mustExec(t, tx,
			`INSERT INTO app.media_assets
			   (kind, storage_key, mime_type, bytes, original_filename, checksum_sha256, uploaded_by, owner_id)
			 VALUES ('image', 'media/limit.png', 'image/png', 10485760, 'limit.png', repeat('x',32)::bytea, $1, $1)`,
			f.adminID)
	})
}

func TestAnOverlongAssetIsRejected(t *testing.T) {
	// §11.1: 5 minutes. 300001 ms is one millisecond over.
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "media_assets_duration_ms_check",
			`INSERT INTO app.media_assets
			   (kind, storage_key, mime_type, bytes, duration_ms, original_filename, checksum_sha256, uploaded_by, owner_id)
			 VALUES ('audio', 'media/long.mp3', 'audio/mpeg', 1024, 300001, 'long.mp3', repeat('x',32)::bytea, $1, $1)`,
			f.adminID)
	})
}

func TestTheKindMustAgreeWithTheMimeType(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "media_assets_kind_matches_mime",
			`INSERT INTO app.media_assets
			   (kind, storage_key, mime_type, bytes, original_filename, checksum_sha256, uploaded_by, owner_id)
			 VALUES ('image', 'media/x.png', 'audio/mpeg', 1024, 'x.png', repeat('x',32)::bytea, $1, $1)`,
			f.adminID)
	})
}

func TestAnUnknownMimeTypeIsRejected(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "media_assets_mime_type_check",
			`INSERT INTO app.media_assets
			   (kind, storage_key, mime_type, bytes, duration_ms, original_filename, checksum_sha256, uploaded_by, owner_id)
			 VALUES ('audio', 'media/x.ogg', 'audio/ogg', 1024, 1000, 'x.ogg', repeat('x',32)::bytea, $1, $1)`,
			f.adminID)
	})
}

func TestTwoAssetsMayShareAChecksum(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		for _, key := range []string{"media/first.mp3", "media/second.mp3"} {
			mustExec(t, tx,
				`INSERT INTO app.media_assets
				   (kind, storage_key, mime_type, bytes, duration_ms, original_filename, checksum_sha256, uploaded_by, owner_id)
				 VALUES ('audio', $1, 'audio/mpeg', 2048, 1000, 'same.mp3', repeat('y',32)::bytea, $2, $2)`,
				key, f.adminID)
		}
	})
}

func TestAValidAudioAssetIsAccepted(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.media_assets
			   (kind, storage_key, mime_type, bytes, duration_ms, original_filename, checksum_sha256, uploaded_by, owner_id)
			 VALUES ('audio', 'media/ok.mp3', 'audio/mpeg', 2048, 120000, 'bai-nghe-1.mp3', repeat('z',32)::bytea, $1, $1)`,
			f.adminID)
	})
}

// ------------------------------------------------------- question bank

// newAudioAsset and newImageAsset put a usable media row in the transaction, so
// the composite-FK tests below have a real asset of each kind to point at.
func newAudioAsset(t *testing.T, tx *sql.Tx, owner string) string {
	t.Helper()
	var id string
	ownerColumn, ownerValue := ownerOfAsset(t, tx)
	if err := tx.QueryRow(
		`INSERT INTO app.media_assets
		        (kind, storage_key, mime_type, bytes, duration_ms,
		         original_filename, checksum_sha256, uploaded_by`+ownerColumn+`)
		 VALUES ('audio', $1, 'audio/mpeg', 1024, 10000, 'nghe.mp3',
		         repeat('a', 32)::bytea, $2`+ownerValue+`)
		 RETURNING id`, "audio/"+randomTag(t)+".mp3", owner,
	).Scan(&id); err != nil {
		t.Fatalf("audio fixture: %v", err)
	}
	return id
}

func ownerOfAsset(t *testing.T, tx *sql.Tx) (column, value string) {
	t.Helper()
	if txHasColumn(t, tx, "media_assets", "owner_id") {
		return ", owner_id", ", $2"
	}
	return "", ""
}

func newImageAsset(t *testing.T, tx *sql.Tx, owner string) string {
	t.Helper()
	var id string
	ownerColumn, ownerValue := ownerOfAsset(t, tx)
	if err := tx.QueryRow(
		`INSERT INTO app.media_assets
		        (kind, storage_key, mime_type, bytes,
		         original_filename, checksum_sha256, uploaded_by`+ownerColumn+`)
		 VALUES ('image', $1, 'image/png', 1024, 'anh.png',
		         repeat('b', 32)::bytea, $2`+ownerValue+`)
		 RETURNING id`, "image/"+randomTag(t)+".png", owner,
	).Scan(&id); err != nil {
		t.Fatalf("image fixture: %v", err)
	}
	return id
}

func randomTag(t *testing.T) string {
	t.Helper()
	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return hex.EncodeToString(nonce)
}

// [D-04] The biconditional, tested in both directions. One direction alone
// would pass with a plain implication and leave the other half unenforced.

func TestNonAudioQuestionCannotCarryAnAudioPolicy(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "questions_audio_policy_iff_audio",
			`INSERT INTO app.questions
			        (type, prompt, points, created_by, owner_id, audio_allow_seek, audio_show_transcript_after)
			 VALUES ('short_answer', 'Không có audio', 1.0, $1, $1, false, true)`, f.adminID)
	})
}

func TestAudioQuestionMustCarryAnAudioPolicy(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		asset := newAudioAsset(t, tx, f.adminID)
		rejectsWith(t, tx, "questions_audio_policy_iff_audio",
			`INSERT INTO app.questions
			        (type, prompt, points, created_by, owner_id, media_asset_id, media_asset_kind)
			 VALUES ('short_answer', 'Có audio nhưng thiếu policy', 1.0, $1, $1, $2, 'audio')`,
			f.adminID, asset)
	})
}

func TestAudioQuestionWithItsPolicyIsAccepted(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		asset := newAudioAsset(t, tx, f.adminID)
		mustExec(t, tx,
			`INSERT INTO app.questions
			        (type, prompt, points, created_by, owner_id, media_asset_id, media_asset_kind,
			         audio_max_plays, audio_allow_seek, audio_show_transcript_after, transcript)
			 VALUES ('short_answer', 'Nghe và trả lời', 2.0, $1, $1, $2, 'audio',
			         2, false, true, 'Hello there.')`, f.adminID, asset)
	})
}

// [D-05] The composite FK is what makes the audio-policy CHECK enforceable
// relationally. Claiming 'audio' for an image asset must fail at the FK, before
// the CHECK ever gets a chance to be satisfied by a lie.
func TestImageAssetCannotBeClaimedAsAudio(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		image := newImageAsset(t, tx, f.adminID)
		rejectsWith(t, tx, "questions_media_asset_id_media_asset_kind_fkey",
			`INSERT INTO app.questions
			        (type, prompt, points, created_by, owner_id, media_asset_id, media_asset_kind,
			         audio_allow_seek, audio_show_transcript_after)
			 VALUES ('short_answer', 'Ảnh giả làm audio', 1.0, $1, $1, $2, 'audio', false, true)`,
			f.adminID, image)
	})
}

// The other half of D-05: an honestly-declared image cannot carry an audio
// policy either. Together these close the loophole in both directions.
func TestImageQuestionCannotCarryAnAudioPolicy(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		image := newImageAsset(t, tx, f.adminID)
		rejectsWith(t, tx, "questions_audio_policy_iff_audio",
			`INSERT INTO app.questions
			        (type, prompt, points, created_by, owner_id, media_asset_id, media_asset_kind,
			         audio_allow_seek, audio_show_transcript_after)
			 VALUES ('short_answer', 'Ảnh có policy audio', 1.0, $1, $1, $2, 'image', false, true)`,
			f.adminID, image)
	})
}

func TestMediaPairMustBeCompleteOrAbsent(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		asset := newAudioAsset(t, tx, f.adminID)
		rejectsWith(t, tx, "questions_media_pair_complete",
			`INSERT INTO app.questions (type, prompt, points, created_by, owner_id, media_asset_id)
			 VALUES ('short_answer', 'Thiếu kind', 1.0, $1, $1, $2)`, f.adminID, asset)
	})
}

func TestSampleAnswerIsForWrittenAnswersOnly(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "questions_sample_answer_only_written",
			`INSERT INTO app.questions (type, prompt, points, created_by, owner_id, sample_answer)
			 VALUES ('single_choice', 'Chọn một đáp án', 1.0, $1, $1, 'Đáp án mẫu')`, f.adminID)
	})
}

func TestSampleAnswerOnShortAnswerIsAccepted(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.questions (type, prompt, points, created_by, owner_id, sample_answer)
			 VALUES ('short_answer', 'Viết câu trả lời', 1.0, $1, $1, 'Đáp án mẫu')`, f.adminID)
	})
}

func TestANewChoiceTypeCannotCarryASampleAnswer(t *testing.T) {
	for _, qtype := range []string{"tfng", "matching"} {
		withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
			rejectsWith(t, tx, "questions_sample_answer_only_written",
				`INSERT INTO app.questions (type, prompt, points, created_by, owner_id, sample_answer, match_allow_reuse)
				 VALUES ($2::app.question_type, 'Câu hỏi', 1.0, $1, $1, 'Đáp án mẫu', CASE WHEN $2 = 'matching' THEN false END)`,
				f.adminID, qtype)
		})
	}
}

func TestSampleAnswerOnAnEssayIsAccepted(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.questions (type, prompt, points, created_by, owner_id, sample_answer)
			 VALUES ('essay', 'Viết bài luận 250 từ', 9.0, $1, $1, 'Bài mẫu')`, f.adminID)
	})
}

func TestTranscriptRequiresAudio(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "questions_transcript_iff_audio",
			`INSERT INTO app.questions (type, prompt, points, created_by, owner_id, transcript)
			 VALUES ('short_answer', 'Không có audio', 1.0, $1, $1, 'Lời thoại')`, f.adminID)
	})
}

func TestBlankOrdinalsStartAtOne(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		q := newQuestion(t, tx, f.adminID, "fill_blank", "Điền vào {{1}}")
		rejectsWith(t, tx, "question_blanks_ordinal_check",
			`INSERT INTO app.question_blanks (question_id, ordinal) VALUES ($1, 0)`, q)
	})
}

func TestBlankAnswersAreCaseDistinct(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		q := newQuestion(t, tx, f.adminID, "fill_blank", "Điền vào {{1}}")
		var blank string
		if err := tx.QueryRow(
			`INSERT INTO app.question_blanks (question_id, ordinal, case_sensitive)
			 VALUES ($1, 1, true) RETURNING id`, q,
		).Scan(&blank); err != nil {
			t.Fatal(err)
		}
		mustExec(t, tx, `INSERT INTO app.question_blank_answers (blank_id, answer) VALUES ($1,'Cat')`, blank)
		mustExec(t, tx, `INSERT INTO app.question_blank_answers (blank_id, answer) VALUES ($1,'cat')`, blank)
		rejectsWith(t, tx, "question_blank_answers_blank_id_answer_key",
			`INSERT INTO app.question_blank_answers (blank_id, answer) VALUES ($1,'Cat')`, blank)
	})
}

// newQuestion inserts a minimal question of the given type.
func newQuestion(t *testing.T, tx *sql.Tx, author, qtype, prompt string) string {
	t.Helper()
	var id string
	if err := tx.QueryRow(
		`INSERT INTO app.questions (type, prompt, points, created_by, owner_id)
		 VALUES ($1::app.question_type, $2, 1.0, $3, $3) RETURNING id`,
		qtype, prompt, author,
	).Scan(&id); err != nil {
		t.Fatalf("question fixture: %v", err)
	}
	return id
}

// [D-04] The half of the audio-policy rule that 00011 left open, closed by
// 00014. `audio_max_plays` is excluded from the biconditional's right-hand side
// on purpose -- maxPlays is nullable, where null means unlimited -- but nothing
// stopped it being SET on a question with no audio at all.
func TestMaxPlaysCannotBeSetWithoutAudio(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "questions_max_plays_only_with_audio",
			`INSERT INTO app.questions (type, prompt, points, created_by, owner_id, audio_max_plays)
			 VALUES ('short_answer', 'Không có audio', 1.0, $1, $1, 3)`, f.adminID)
	})
}

func TestMaxPlaysCannotBeSetOnAnImageQuestion(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		image := newImageAsset(t, tx, f.adminID)
		rejectsWith(t, tx, "questions_max_plays_only_with_audio",
			`INSERT INTO app.questions
			        (type, prompt, points, created_by, owner_id, media_asset_id, media_asset_kind, audio_max_plays)
			 VALUES ('short_answer', 'Ảnh', 1.0, $1, $1, $2, 'image', 3)`, f.adminID, image)
	})
}

// NULL still means unlimited on an audio question -- the reason maxPlays was
// left out of the biconditional in the first place. A constraint that broke
// this would forbid "unlimited plays", which §7 allows.
func TestAnAudioQuestionMayLeaveMaxPlaysUnlimited(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		asset := newAudioAsset(t, tx, f.adminID)
		mustExec(t, tx,
			`INSERT INTO app.questions
			        (type, prompt, points, created_by, owner_id, media_asset_id, media_asset_kind,
			         audio_allow_seek, audio_show_transcript_after, audio_max_plays)
			 VALUES ('short_answer', 'Nghe không giới hạn', 1.0, $1, $1, $2, 'audio', false, true, NULL)`,
			f.adminID, asset)
	})
}

func TestAnAudioQuestionMaySetMaxPlays(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		asset := newAudioAsset(t, tx, f.adminID)
		mustExec(t, tx,
			`INSERT INTO app.questions
			        (type, prompt, points, created_by, owner_id, media_asset_id, media_asset_kind,
			         audio_allow_seek, audio_show_transcript_after, audio_max_plays)
			 VALUES ('short_answer', 'Nghe tối đa 2 lần', 1.0, $1, $1, $2, 'audio', false, true, 2)`,
			f.adminID, asset)
	})
}

// ---------------------------------------------------- tests and drafts

// [D-14] §7's `currentVersion` has to mean something: a published or archived
// test with no snapshot behind it is an assignment pointing at nothing.
func TestAPublishedTestMustHaveAVersion(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "tests_published_has_version",
			`INSERT INTO app.tests (title, status, current_version, created_by, owner_id)
			 VALUES ('Đề đã xuất bản', 'published', 0, $1, $1)`, f.adminID)
	})
}

// 00027: only a published test can be pointed at by an assignment, so only
// that status needs a snapshot behind it. An abandoned draft archives as it is.
func TestAnArchivedDraftNeedsNoVersion(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.tests (title, status, current_version, created_by, owner_id)
			 VALUES ('Đề lưu trữ', 'archived', 0, $1, $1)`, f.adminID)
	})
}

func TestADraftMayHaveNoVersionYet(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.tests (title, created_by, owner_id) VALUES ('Đề nháp', $1, $1)`, f.adminID)
	})
}

func TestAPublishedTestWithAVersionIsAccepted(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.tests (title, status, current_version, created_by, owner_id)
			 VALUES ('Đề đã xuất bản', 'published', 1, $1, $1)`, f.adminID)
	})
}

// A draft references a bank question, it does not own one. The bank
// soft-deletes, so RESTRICT fires only on a hard purge -- which is exactly when
// someone should be told a draft still uses it.
func TestHardDeletingAQuestionADraftUsesIsBlocked(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		q := newQuestion(t, tx, f.adminID, "short_answer", "Câu hỏi trong đề nháp")
		section := newSection(t, tx, f.adminID)
		mustExec(t, tx,
			`INSERT INTO app.test_section_questions (test_section_id, ordinal, question_id)
			 VALUES ($1, 0, $2)`, section, q)

		rejectsWith(t, tx, "test_section_questions_question_id_fkey",
			`DELETE FROM app.questions WHERE id = $1`, q)
	})
}

// Sections and their question rows ARE owned by the test, so dropping the test
// takes them with it.
func TestDeletingATestCascadesToItsDraftStructure(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		q := newQuestion(t, tx, f.adminID, "short_answer", "Câu hỏi bị cuốn theo")
		section := newSection(t, tx, f.adminID)
		mustExec(t, tx,
			`INSERT INTO app.test_section_questions (test_section_id, ordinal, question_id)
			 VALUES ($1, 0, $2)`, section, q)

		var testID string
		if err := tx.QueryRow(
			`SELECT test_id::text FROM app.test_sections WHERE id = $1`, section,
		).Scan(&testID); err != nil {
			t.Fatal(err)
		}
		mustExec(t, tx, `DELETE FROM app.tests WHERE id = $1`, testID)

		var sections, links int
		if err := tx.QueryRow(
			`SELECT (SELECT count(*) FROM app.test_sections WHERE test_id = $1),
			        (SELECT count(*) FROM app.test_section_questions WHERE test_section_id = $2)`,
			testID, section,
		).Scan(&sections, &links); err != nil {
			t.Fatal(err)
		}
		if sections != 0 || links != 0 {
			t.Errorf("%d sections and %d question links survived the test's deletion", sections, links)
		}

		// The question itself is not owned by the test and must remain.
		var questions int
		if err := tx.QueryRow(`SELECT count(*) FROM app.questions WHERE id = $1`, q).Scan(&questions); err != nil {
			t.Fatal(err)
		}
		if questions != 1 {
			t.Error("deleting a test took a bank question with it")
		}
	})
}

func TestOneQuestionCannotAppearTwiceInASection(t *testing.T) {
	// Two copies would be indistinguishable to a student and would score twice.
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		q := newQuestion(t, tx, f.adminID, "short_answer", "Câu hỏi lặp")
		section := newSection(t, tx, f.adminID)
		mustExec(t, tx,
			`INSERT INTO app.test_section_questions (test_section_id, ordinal, question_id)
			 VALUES ($1, 0, $2)`, section, q)

		rejectsWith(t, tx, "test_section_questions_no_dupes",
			`INSERT INTO app.test_section_questions (test_section_id, ordinal, question_id)
			 VALUES ($1, 1, $2)`, section, q)
	})
}

// [D-13] The draft-editable ordinal uniques are deferrable for the same reason
// the bank's are: the builder reorders one row at a time.
func TestSectionOrdinalsAreDeferrable(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		section := newSection(t, tx, f.adminID)
		var testID string
		if err := tx.QueryRow(
			`SELECT test_id::text FROM app.test_sections WHERE id = $1`, section,
		).Scan(&testID); err != nil {
			t.Fatal(err)
		}
		mustExec(t, tx,
			`INSERT INTO app.test_sections (test_id, ordinal, title) VALUES ($1, 1, 'Phần 2')`, testID)

		rejectsWith(t, tx, "test_sections_ordinal_key",
			`UPDATE app.test_sections SET ordinal = 1 WHERE test_id = $1 AND ordinal = 0`, testID)
	})
}

// newSection creates a draft test with one section and returns the section id.
func newSection(t *testing.T, tx *sql.Tx, author string) string {
	t.Helper()
	var testID, sectionID string
	if err := tx.QueryRow(
		`INSERT INTO app.tests (title, created_by, owner_id) VALUES ('Đề nháp', $1, $1) RETURNING id`,
		author,
	).Scan(&testID); err != nil {
		t.Fatalf("test fixture: %v", err)
	}
	if err := tx.QueryRow(
		`INSERT INTO app.test_sections (test_id, ordinal, title)
		 VALUES ($1, 0, 'Phần 1') RETURNING id`, testID,
	).Scan(&sectionID); err != nil {
		t.Fatalf("section fixture: %v", err)
	}
	return sectionID
}

// ------------------------------------------------- version snapshots

func txHasColumn(t *testing.T, tx *sql.Tx, table, column string) bool {
	t.Helper()
	var exists bool
	if err := tx.QueryRow(`SELECT EXISTS (SELECT 1 FROM pg_attribute
		WHERE attrelid = ('app.' || $1)::regclass AND attname = $2 AND NOT attisdropped)`, table, column).Scan(&exists); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return exists
}

// newVersion creates a published test with one version and one section, and
// returns the section id.
func newVersion(t *testing.T, tx *sql.Tx, author string) string {
	t.Helper()
	var testID, versionID, sectionID string
	ownerColumn, ownerValue := "", ""
	if txHasColumn(t, tx, "tests", "owner_id") {
		ownerColumn, ownerValue = ", owner_id", ", $1"
	}
	if err := tx.QueryRow(
		`INSERT INTO app.tests (title, status, current_version, created_by`+ownerColumn+`)
		 VALUES ('Đề đã xuất bản', 'published', 1, $1`+ownerValue+`) RETURNING id`, author,
	).Scan(&testID); err != nil {
		t.Fatalf("test fixture: %v", err)
	}
	if err := tx.QueryRow(
		`INSERT INTO app.test_versions (test_id, version, total_points, published_by)
		 VALUES ($1, 1, 10.00, $2) RETURNING id`, testID, author,
	).Scan(&versionID); err != nil {
		t.Fatalf("version fixture: %v", err)
	}
	if err := tx.QueryRow(
		`INSERT INTO app.test_version_sections (test_version_id, ordinal, title)
		 VALUES ($1, 0, 'Phần 1') RETURNING id`, versionID,
	).Scan(&sectionID); err != nil {
		t.Fatalf("version section fixture: %v", err)
	}
	return sectionID
}

// [D-07] source_question_id is ON DELETE SET NULL rather than RESTRICT: it is
// informational, and RESTRICT would let a three-year-old frozen version veto a
// bank cleanup forever. Losing it degrades a link, not an attempt -- so the
// version row and everything a student sat must survive intact.
func TestHardDeletingABankQuestionNullsTheVersionLinkAndKeepsTheSnapshot(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		q := newQuestion(t, tx, f.adminID, "short_answer", "Câu hỏi gốc")
		section := newVersion(t, tx, f.adminID)

		var vqID string
		if err := tx.QueryRow(
			`INSERT INTO app.test_version_questions
			        (test_version_section_id, ordinal, source_question_id, type, prompt, points)
			 VALUES ($1, 0, $2, 'short_answer', 'Bản đông cứng', 4.00) RETURNING id`,
			section, q,
		).Scan(&vqID); err != nil {
			t.Fatal(err)
		}

		mustExec(t, tx, `DELETE FROM app.questions WHERE id = $1`, q)

		var source *string
		var prompt string
		var points string
		if err := tx.QueryRow(
			`SELECT source_question_id::text, prompt, points::text
			   FROM app.test_version_questions WHERE id = $1`, vqID,
		).Scan(&source, &prompt, &points); err != nil {
			t.Fatalf("the version question was deleted with the bank question: %v", err)
		}
		if source != nil {
			t.Errorf("source_question_id is %q, want NULL after the bank question was purged", *source)
		}
		if prompt != "Bản đông cứng" || points != "4.00" {
			t.Errorf("the snapshot changed: prompt=%q points=%q", prompt, points)
		}
	})
}

// [D-17] The composite-FK target assignments will use to prove a version
// belongs to the test it names.
func TestAVersionCarriesTheCompositeKeyAssignmentsWillNeed(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		newVersion(t, tx, f.adminID)

		var exists bool
		if err := tx.QueryRow(
			`SELECT EXISTS (
			    SELECT 1 FROM pg_constraint
			     WHERE conname = 'test_versions_id_test_key' AND contype = 'u')`,
		).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Error("test_versions_id_test_key is missing; assignments cannot prove a version " +
				"belongs to its test without it")
		}
	})
}

// A version is a historical fact. Deleting the test it belongs to must fail
// here rather than two levels down inside attempts.
func TestHardDeletingATestWithAVersionIsRestricted(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		section := newVersion(t, tx, f.adminID)
		var testID string
		if err := tx.QueryRow(
			`SELECT v.test_id::text FROM app.test_version_sections s
			   JOIN app.test_versions v ON v.id = s.test_version_id
			  WHERE s.id = $1`, section,
		).Scan(&testID); err != nil {
			t.Fatal(err)
		}

		rejectsWith(t, tx, "test_versions_test_id_fkey",
			`DELETE FROM app.tests WHERE id = $1`, testID)
	})
}

// An asset a published version depends on must not vanish under it.
func TestHardDeletingAnAssetAVersionUsesIsRestricted(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		asset := newAudioAsset(t, tx, f.adminID)
		section := newVersion(t, tx, f.adminID)
		mustExec(t, tx,
			`INSERT INTO app.test_version_questions
			        (test_version_section_id, ordinal, type, prompt, points,
			         media_asset_id, media_asset_kind, audio_allow_seek, audio_show_transcript_after)
			 VALUES ($1, 0, 'short_answer', 'Nghe', 4.00, $2, 'audio', false, true)`, section, asset)

		rejectsWith(t, tx, "test_version_questions_media_asset_id_media_asset_kind_fkey",
			`DELETE FROM app.media_assets WHERE id = $1`, asset)
	})
}

// The same [D-04] biconditional the bank carries, on the frozen copy: a version
// question is what a student actually sits, so the audio rule matters more here
// rather than less.
func TestAVersionQuestionCannotCarryAnAudioPolicyWithoutAudio(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		section := newVersion(t, tx, f.adminID)
		rejectsWith(t, tx, "tvq_audio_policy_iff_audio",
			`INSERT INTO app.test_version_questions
			        (test_version_section_id, ordinal, type, prompt, points,
			         audio_allow_seek, audio_show_transcript_after)
			 VALUES ($1, 0, 'short_answer', 'Không có audio', 4.00, false, true)`, section)
	})
}

func TestAVersionQuestionCannotSetMaxPlaysWithoutAudio(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		section := newVersion(t, tx, f.adminID)
		rejectsWith(t, tx, "tvq_max_plays_only_with_audio",
			`INSERT INTO app.test_version_questions
			        (test_version_section_id, ordinal, type, prompt, points, audio_max_plays)
			 VALUES ($1, 0, 'short_answer', 'Không có audio', 4.00, 3)`, section)
	})
}

func TestAVersionMustHaveAPositiveVersionNumber(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		var testID string
		if err := tx.QueryRow(
			`INSERT INTO app.tests (title, created_by, owner_id) VALUES ('Đề', $1, $1) RETURNING id`,
			f.adminID,
		).Scan(&testID); err != nil {
			t.Fatal(err)
		}
		rejectsWith(t, tx, "test_versions_version_check",
			`INSERT INTO app.test_versions (test_id, version, total_points, published_by)
			 VALUES ($1, 0, 10.00, $2)`, testID, f.adminID)
	})
}

var notValidatedConstraints = map[string]string{}

func TestNoConstraintIsLeftNotValidatedOutsideThePinnedList(t *testing.T) {
	conn := migrated(t)
	rows, err := conn.Query(`
		SELECT n.nspname || '.' || r.relname || '.' || c.conname
		  FROM pg_constraint c
		  JOIN pg_class r ON r.oid = c.conrelid
		  JOIN pg_namespace n ON n.oid = r.relnamespace
		 WHERE n.nspname = 'app' AND NOT c.convalidated`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	found := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		found[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for name := range found {
		if _, listed := notValidatedConstraints[name]; !listed {
			t.Errorf("%s is NOT VALID and not on the pinned list: validate it in its contract step, or list it with the step that will", name)
		}
	}
	for name, step := range notValidatedConstraints {
		if !found[name] {
			t.Errorf("%s is on the pinned list (%s) but is validated or gone: remove the line", name, step)
		}
	}
}

func TestAnInsertWithoutAnOwnerIsRefused(t *testing.T) {
	for _, c := range []struct{ table, column, insert string }{
		{"tests", "owner_id", `INSERT INTO app.tests (title, created_by) VALUES ('Không chủ', $1)`},
		{"questions", "owner_id", `INSERT INTO app.questions (type, prompt, points, created_by) VALUES ('short_answer', 'Không chủ', 1.0, $1)`},
		{"question_groups", "owner_id", `INSERT INTO app.question_groups (title, created_by) VALUES ('Không chủ', $1)`},
		{"media_assets", "owner_id", `INSERT INTO app.media_assets
			   (kind, storage_key, mime_type, bytes, original_filename, checksum_sha256, uploaded_by)
			 VALUES ('image', 'media/khong-chu.png', 'image/png', 1024, 'khong-chu.png', repeat('x',32)::bytea, $1)`},
		{"classes", "teacher_id", `INSERT INTO app.classes (name) VALUES ('Không chủ ' || $1::text)`},
	} {
		t.Run(c.table, func(t *testing.T) {
			withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
				rejectsWith(t, tx, `null value in column "`+c.column+`" of relation "`+c.table+`"`, c.insert, f.adminID)
			})
		})
	}
}

// ---------------------------------------------------------------- R6 question types (T-R6.1a)

func TestASelectCountNeedsAMultipleChoice(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "questions_select_count_check",
			`INSERT INTO app.questions (type, prompt, points, created_by, owner_id, select_count)
			 VALUES ('single_choice', 'Chọn một đáp án', 1.0, $1, $1, 2)`, f.adminID)
	})
}

func TestASelectCountStaysBelowTheOptionCap(t *testing.T) {
	for _, n := range []int{1, 8} {
		withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
			rejectsWith(t, tx, "questions_select_count_check",
				`INSERT INTO app.questions (type, prompt, points, created_by, owner_id, select_count)
				 VALUES ('multiple_choice', 'Chọn N đáp án', 1.0, $1, $1, $2)`, f.adminID, n)
		})
	}
}

func TestAChooseTwoQuestionIsAccepted(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.questions (type, prompt, points, created_by, owner_id, select_count)
			 VALUES ('multiple_choice', 'Chọn HAI đáp án', 2.0, $1, $1, 2)`, f.adminID)
	})
}

func TestAWordLimitOfZeroIsRefused(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "questions_max_words_check",
			`INSERT INTO app.questions (type, prompt, points, created_by, owner_id, max_words)
			 VALUES ('short_answer', 'Không quá 0 từ', 1.0, $1, $1, 0)`, f.adminID)
	})
}

func TestAWordLimitIsForShortAnswersOnly(t *testing.T) {
	for _, qtype := range []string{"essay", "fill_blank"} {
		withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
			rejectsWith(t, tx, "questions_max_words_check",
				`INSERT INTO app.questions (type, prompt, points, created_by, owner_id, max_words)
				 VALUES ($2::app.question_type, 'Không quá 50 từ', 1.0, $1, $1, 50)`, f.adminID, qtype)
		})
	}
}

func TestAWordLimitStopsAtAHundred(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "questions_max_words_check",
			`INSERT INTO app.questions (type, prompt, points, created_by, owner_id, max_words)
			 VALUES ('short_answer', 'Không quá 101 từ', 1.0, $1, $1, 101)`, f.adminID)
	})
}

func TestAShortAnswerOfThreeWordsIsAccepted(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.questions (type, prompt, points, created_by, owner_id, max_words)
			 VALUES ('short_answer', 'Không quá BA từ', 1.0, $1, $1, 3)`, f.adminID)
	})
}

func TestAVersionQuestionKeepsTheSameRules(t *testing.T) {
	for _, c := range []struct {
		name, constraint, column, qtype string
		value                           int
	}{
		{"select count on a single choice", "test_version_questions_select_count_check", "select_count", "single_choice", 3},
		{"select count of one", "test_version_questions_select_count_check", "select_count", "multiple_choice", 1},
		{"select count of eight", "test_version_questions_select_count_check", "select_count", "multiple_choice", 8},
		{"word limit on a multiple choice", "test_version_questions_max_words_check", "max_words", "multiple_choice", 3},
		{"word limit of zero", "test_version_questions_max_words_check", "max_words", "short_answer", 0},
		{"word limit of 101", "test_version_questions_max_words_check", "max_words", "short_answer", 101},
	} {
		t.Run(c.name, func(t *testing.T) {
			withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
				section := newVersion(t, tx, f.adminID)
				rejectsWith(t, tx, c.constraint,
					`INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, points, `+c.column+`)
					 VALUES ($1, 0, $2::app.question_type, 'Câu đã chốt', 1.0, $3)`, section, c.qtype, c.value)
			})
		})
	}
}

func newBankGroup(t *testing.T, tx *sql.Tx, author string) string {
	t.Helper()
	var id string
	if err := tx.QueryRow(`INSERT INTO app.question_groups (title, created_by, owner_id)
		VALUES ('Bài đọc', $1, $1) RETURNING id`, author).Scan(&id); err != nil {
		t.Fatalf("group fixture: %v", err)
	}
	return id
}

func TestATfngOrMatchingGroupMemberMayKeepAFixedOrder(t *testing.T) {
	for _, qtype := range []string{"tfng", "matching"} {
		withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
			group := newBankGroup(t, tx, f.adminID)
			mustExec(t, tx, `INSERT INTO app.questions
				(type, prompt, points, created_by, owner_id, context_group_id, context_ordinal, context_option_order, match_allow_reuse)
				VALUES ($2::app.question_type, 'Câu trong nhóm', 1.0, $1, $1, $3, 0, 'fixed', CASE WHEN $2 = 'matching' THEN false END)`,
				f.adminID, qtype, group)
		})
	}
}

func TestAnEssayGroupMemberCannotKeepAFixedOrder(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		group := newBankGroup(t, tx, f.adminID)
		rejectsWith(t, tx, "questions_context_option_order",
			`INSERT INTO app.questions
			   (type, prompt, points, created_by, owner_id, context_group_id, context_ordinal, context_option_order)
			 VALUES ('essay', 'Bài luận trong nhóm', 9.0, $1, $1, $2, 0, 'fixed')`, f.adminID, group)
	})
}

// TestTheR6ChecksAreValidated fails when a migration adds one of its CHECKs NOT
// VALID and skips the VALIDATE.
func TestTheR6ChecksAreValidated(t *testing.T) {
	conn := migrated(t)
	for _, name := range []string{
		"questions_select_count_check", "test_version_questions_select_count_check",
		"questions_max_words_check", "test_version_questions_max_words_check",
		"questions_sample_answer_only_written", "questions_context_option_order",
	} {
		var validated bool
		if err := conn.QueryRow(`SELECT convalidated FROM pg_constraint
			WHERE conname = $1 AND connamespace = 'app'::regnamespace`, name).Scan(&validated); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !validated {
			t.Errorf("%s is NOT VALID", name)
		}
	}
}

// TestTheV0120WritesStillInsert runs the bank's and the publish's inserts with
// the column lists of v0.12.0 (questions/repositories/store.go and
// tests/repositories/publish_snapshot.go before R6), which an old binary sends
// while the new schema is live.
func TestTheV0120WritesStillInsert(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		for _, qtype := range []string{"single_choice", "multiple_choice", "true_false", "fill_blank", "short_answer"} {
			mustExec(t, tx, `
				INSERT INTO app.questions
				       (type, prompt, media_asset_id, media_asset_kind,
				        audio_max_plays, audio_allow_seek, audio_show_transcript_after,
				        transcript, points, explanation, sample_answer, tags, created_by, prompt_content, explanation_content,
				        id, context_group_id, context_ordinal, context_option_order, owner_id, level, skill, media_alt,
				        audio_allow_pause, copied_from_id)
				VALUES ($1::app.question_type, 'Câu hỏi cũ', NULL, NULL, NULL, NULL, NULL,
				        NULL, 1.0, NULL, CASE WHEN $1 = 'short_answer' THEN 'Đáp án mẫu' END, '{}', $2, NULL, NULL,
				        uuidv7(), NULL, NULL, NULL, $2, NULL, NULL, NULL, NULL, NULL)`, qtype, f.adminID)
		}
		section := newVersion(t, tx, f.adminID)
		mustExec(t, tx, `
			INSERT INTO app.test_version_questions
			       (test_version_section_id, ordinal, source_question_id, type, prompt,
			        media_asset_id, media_asset_kind, audio_max_plays, audio_allow_seek,
			        audio_show_transcript_after, transcript, points, explanation, sample_answer, prompt_content, explanation_content, level, skill, media_alt,
			        audio_allow_pause)
			VALUES ($1, 0, NULL, 'multiple_choice', 'Câu đã chốt', NULL, NULL, NULL, NULL, NULL,
			        NULL, 1.0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL)`, section)
	})
}

// ---------------------------------------------------------------- rubrics, matching and bands (T-R6.1b)

func newMatchingQuestion(t *testing.T, tx *sql.Tx, author string) (question string, options []string) {
	t.Helper()
	if err := tx.QueryRow(
		`INSERT INTO app.questions (type, prompt, points, created_by, owner_id, match_allow_reuse)
		 VALUES ('matching', 'Nối tiêu đề với đoạn văn', 2.0, $1, $1, false) RETURNING id`, author,
	).Scan(&question); err != nil {
		t.Fatalf("matching fixture: %v", err)
	}
	for i, text := range []string{"Tiêu đề i", "Tiêu đề ii"} {
		var option string
		if err := tx.QueryRow(
			`INSERT INTO app.question_options (question_id, ordinal, text) VALUES ($1, $2, $3) RETURNING id`,
			question, i, text,
		).Scan(&option); err != nil {
			t.Fatalf("option fixture: %v", err)
		}
		options = append(options, option)
	}
	return question, options
}

func TestAMatchingQuestionMustSayWhetherChoicesRepeat(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "questions_match_allow_reuse_iff_matching",
			`INSERT INTO app.questions (type, prompt, points, created_by, owner_id)
			 VALUES ('matching', 'Nối', 1.0, $1, $1)`, f.adminID)
	})
}

func TestOnlyAMatchingQuestionSaysWhetherChoicesRepeat(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "questions_match_allow_reuse_iff_matching",
			`INSERT INTO app.questions (type, prompt, points, created_by, owner_id, match_allow_reuse)
			 VALUES ('single_choice', 'Chọn một', 1.0, $1, $1, true)`, f.adminID)
	})
}

func TestAMatchItemCannotBeKeyedToAnotherQuestionsOption(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		question, _ := newMatchingQuestion(t, tx, f.adminID)
		_, others := newMatchingQuestion(t, tx, f.adminID)
		rejectsWith(t, tx, "question_match_items_option_fk",
			`INSERT INTO app.question_match_items (question_id, ordinal, text, correct_option_id)
			 VALUES ($1, 0, 'Đoạn A', $2)`, question, others[0])
	})
}

func TestAnOptionAnItemNamesCannotBeDeleted(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		question, options := newMatchingQuestion(t, tx, f.adminID)
		mustExec(t, tx, `INSERT INTO app.question_match_items (question_id, ordinal, text, correct_option_id)
			VALUES ($1, 0, 'Đoạn A', $2)`, question, options[0])
		rejectsWith(t, tx, "question_match_items_option_fk",
			`DELETE FROM app.question_options WHERE id = $1`, options[0])
	})
}

// The key is NO ACTION, checked at the end of the statement, so a purge that
// cascades to the options and the items together passes in either order.
func TestPurgingAMatchingQuestionTakesItsItemsAndOptions(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		question, options := newMatchingQuestion(t, tx, f.adminID)
		for i, option := range options {
			mustExec(t, tx, `INSERT INTO app.question_match_items (question_id, ordinal, text, correct_option_id)
				VALUES ($1, $2, 'Đoạn', $3)`, question, i, option)
		}
		mustExec(t, tx, `DELETE FROM app.questions WHERE id = $1`, question)
		var left int
		if err := tx.QueryRow(`SELECT (SELECT count(*) FROM app.question_match_items WHERE question_id = $1)
			+ (SELECT count(*) FROM app.question_options WHERE question_id = $1)`, question).Scan(&left); err != nil {
			t.Fatal(err)
		}
		if left != 0 {
			t.Errorf("%d items and options outlived their question", left)
		}
	})
}

func TestAFrozenMatchItemCannotBeKeyedToAnotherQuestionsOption(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		section := newVersion(t, tx, f.adminID)
		var first, second, foreign string
		for ordinal, dest := range []*string{&first, &second} {
			if err := tx.QueryRow(`INSERT INTO app.test_version_questions
				(test_version_section_id, ordinal, type, prompt, points, match_allow_reuse)
				VALUES ($1, $2, 'matching', 'Nối', 1.0, false) RETURNING id`, section, ordinal).Scan(dest); err != nil {
				t.Fatal(err)
			}
		}
		if err := tx.QueryRow(`INSERT INTO app.test_version_options (test_version_question_id, ordinal, text)
			VALUES ($1, 0, 'Tiêu đề i') RETURNING id`, second).Scan(&foreign); err != nil {
			t.Fatal(err)
		}
		rejectsWith(t, tx, "test_version_match_items_option_fk",
			`INSERT INTO app.test_version_match_items (test_version_question_id, ordinal, text, correct_option_id)
			 VALUES ($1, 0, 'Đoạn A', $2)`, first, foreign)
	})
}

func TestARubricCriterionNeedsALabel(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		essay := newQuestion(t, tx, f.adminID, "essay", "Viết bài luận")
		rejectsWith(t, tx, "question_rubric_criteria_label_check",
			`INSERT INTO app.question_rubric_criteria (question_id, ordinal, label) VALUES ($1, 0, '   ')`, essay)
	})
}

// gradedEssay freezes an essay with one rubric criterion, starts an attempt on
// its version and saves an answer, returning the attempt, the frozen question
// and the criterion.
func gradedEssay(t *testing.T, tx *sql.Tx, f fixture) (attempt, question, criterion string) {
	t.Helper()
	section := newVersion(t, tx, f.adminID)
	if err := tx.QueryRow(`INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, points)
		VALUES ($1, 0, 'essay', 'Viết bài luận', 9.0) RETURNING id`, section).Scan(&question); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(`INSERT INTO app.test_version_rubric_criteria (test_version_question_id, ordinal, label)
		VALUES ($1, 0, 'Task response') RETURNING id`, question).Scan(&criterion); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(`
		WITH assignment AS (
		  INSERT INTO app.assignments (test_id, test_version_id, opens_at, closes_at, duration_minutes, created_by)
		  SELECT v.test_id, v.id, now() - interval '1 hour', now() + interval '1 hour', 30, $2
		    FROM app.test_version_sections s JOIN app.test_versions v ON v.id = s.test_version_id
		   WHERE s.id = $1
		  RETURNING id, test_version_id)
		INSERT INTO app.attempts (assignment_id, test_version_id, student_id, attempt_no, session_id, shuffle_seed,
		                          beacon_token_hash, started_at, deadline_at)
		SELECT id, test_version_id, $3, 1, uuidv7(), 1, sha256('beacon'::bytea), now(), now() + interval '30 minutes'
		  FROM assignment RETURNING id`, section, f.adminID, f.studentID).Scan(&attempt); err != nil {
		t.Fatal(err)
	}
	mustExec(t, tx, `INSERT INTO app.attempt_answers (attempt_id, question_id, payload, requires_manual)
		VALUES ($1, $2, '{"type": "text", "value": "Bài viết"}', true)`, attempt, question)
	return attempt, question, criterion
}

func TestABandMovesInHalfSteps(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		attempt, question, _ := gradedEssay(t, tx, f)
		rejectsWith(t, tx, "attempt_answers_band_step",
			`UPDATE app.attempt_answers SET band = 6.25, manual_score = 6.25, graded_at = now()
			 WHERE attempt_id = $1 AND question_id = $2`, attempt, question)
	})
}

func TestABandNeedsTheScoreItConvertsTo(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		attempt, question, _ := gradedEssay(t, tx, f)
		rejectsWith(t, tx, "attempt_answers_band_needs_score",
			`UPDATE app.attempt_answers SET band = 6.5 WHERE attempt_id = $1 AND question_id = $2`, attempt, question)
	})
}

func TestAGradedBandIsAccepted(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		attempt, question, criterion := gradedEssay(t, tx, f)
		mustExec(t, tx, `UPDATE app.attempt_answers SET band = 6.5, manual_score = 6.50, graded_at = now()
			WHERE attempt_id = $1 AND question_id = $2`, attempt, question)
		mustExec(t, tx, `INSERT INTO app.attempt_answer_criteria (attempt_id, question_id, criterion_id, band)
			VALUES ($1, $2, $3, 7.0)`, attempt, question, criterion)
	})
}

func TestACriterionBandMovesInHalfSteps(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		attempt, question, criterion := gradedEssay(t, tx, f)
		rejectsWith(t, tx, "attempt_answer_criteria_band_step",
			`INSERT INTO app.attempt_answer_criteria (attempt_id, question_id, criterion_id, band)
			 VALUES ($1, $2, $3, 9.5)`, attempt, question, criterion)
	})
}

func TestAGradeCannotNameAnotherQuestionsCriterion(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		attempt, question, _ := gradedEssay(t, tx, f)
		_, _, foreign := gradedEssay(t, tx, f)
		rejectsWith(t, tx, "attempt_answer_criteria_criterion_fk",
			`INSERT INTO app.attempt_answer_criteria (attempt_id, question_id, criterion_id, band)
			 VALUES ($1, $2, $3, 7.0)`, attempt, question, foreign)
	})
}
