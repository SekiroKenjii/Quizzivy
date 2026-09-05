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
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, created_by)
			 VALUES ($1, sha256('c'::bytea), 'AB12', now() + interval '30 days', $2)`,
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
			   (kind, storage_key, mime_type, bytes, original_filename, checksum_sha256, uploaded_by)
			 VALUES ('audio', 'media/a.mp3', 'audio/mpeg', 1024, 'a.mp3', repeat('x',32)::bytea, $1)`,
			f.adminID)
	})
}

func TestANonAudioAssetMustNotCarryADuration(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "media_assets_audio_has_duration",
			`INSERT INTO app.media_assets
			   (kind, storage_key, mime_type, bytes, duration_ms, original_filename, checksum_sha256, uploaded_by)
			 VALUES ('image', 'media/i.png', 'image/png', 1024, 5000, 'i.png', repeat('x',32)::bytea, $1)`,
			f.adminID)
	})
}

func TestAnOversizedAssetIsRejected(t *testing.T) {
	// §11.1: 10 MB. 10485761 is one byte over.
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "media_assets_bytes_check",
			`INSERT INTO app.media_assets
			   (kind, storage_key, mime_type, bytes, duration_ms, original_filename, checksum_sha256, uploaded_by)
			 VALUES ('audio', 'media/big.mp3', 'audio/mpeg', 10485761, 1000, 'big.mp3', repeat('x',32)::bytea, $1)`,
			f.adminID)
	})
}

func TestAnOverlongAssetIsRejected(t *testing.T) {
	// §11.1: 5 minutes. 300001 ms is one millisecond over.
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "media_assets_duration_ms_check",
			`INSERT INTO app.media_assets
			   (kind, storage_key, mime_type, bytes, duration_ms, original_filename, checksum_sha256, uploaded_by)
			 VALUES ('audio', 'media/long.mp3', 'audio/mpeg', 1024, 300001, 'long.mp3', repeat('x',32)::bytea, $1)`,
			f.adminID)
	})
}

func TestTheKindMustAgreeWithTheMimeType(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "media_assets_kind_matches_mime",
			`INSERT INTO app.media_assets
			   (kind, storage_key, mime_type, bytes, original_filename, checksum_sha256, uploaded_by)
			 VALUES ('image', 'media/x.png', 'audio/mpeg', 1024, 'x.png', repeat('x',32)::bytea, $1)`,
			f.adminID)
	})
}

func TestAnUnknownMimeTypeIsRejected(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "media_assets_mime_type_check",
			`INSERT INTO app.media_assets
			   (kind, storage_key, mime_type, bytes, duration_ms, original_filename, checksum_sha256, uploaded_by)
			 VALUES ('audio', 'media/x.ogg', 'audio/ogg', 1024, 1000, 'x.ogg', repeat('x',32)::bytea, $1)`,
			f.adminID)
	})
}

func TestTwoAssetsMayShareAChecksum(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		for _, key := range []string{"media/first.mp3", "media/second.mp3"} {
			mustExec(t, tx,
				`INSERT INTO app.media_assets
				   (kind, storage_key, mime_type, bytes, duration_ms, original_filename, checksum_sha256, uploaded_by)
				 VALUES ('audio', $1, 'audio/mpeg', 2048, 1000, 'same.mp3', repeat('y',32)::bytea, $2)`,
				key, f.adminID)
		}
	})
}

func TestAValidAudioAssetIsAccepted(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.media_assets
			   (kind, storage_key, mime_type, bytes, duration_ms, original_filename, checksum_sha256, uploaded_by)
			 VALUES ('audio', 'media/ok.mp3', 'audio/mpeg', 2048, 120000, 'bai-nghe-1.mp3', repeat('z',32)::bytea, $1)`,
			f.adminID)
	})
}

// ------------------------------------------------------- question bank

// newAudioAsset and newImageAsset put a usable media row in the transaction, so
// the composite-FK tests below have a real asset of each kind to point at.
func newAudioAsset(t *testing.T, tx *sql.Tx, owner string) string {
	t.Helper()
	var id string
	if err := tx.QueryRow(
		`INSERT INTO app.media_assets
		        (kind, storage_key, mime_type, bytes, duration_ms,
		         original_filename, checksum_sha256, uploaded_by)
		 VALUES ('audio', $1, 'audio/mpeg', 1024, 10000, 'nghe.mp3',
		         repeat('a', 32)::bytea, $2)
		 RETURNING id`, "audio/"+randomTag(t)+".mp3", owner).Scan(&id); err != nil {
		t.Fatalf("audio fixture: %v", err)
	}
	return id
}

func newImageAsset(t *testing.T, tx *sql.Tx, owner string) string {
	t.Helper()
	var id string
	if err := tx.QueryRow(
		`INSERT INTO app.media_assets
		        (kind, storage_key, mime_type, bytes,
		         original_filename, checksum_sha256, uploaded_by)
		 VALUES ('image', $1, 'image/png', 1024, 'anh.png',
		         repeat('b', 32)::bytea, $2)
		 RETURNING id`, "image/"+randomTag(t)+".png", owner).Scan(&id); err != nil {
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
			        (type, prompt, points, created_by, audio_allow_seek, audio_show_transcript_after)
			 VALUES ('short_answer', 'Không có audio', 1.0, $1, false, true)`, f.adminID)
	})
}

func TestAudioQuestionMustCarryAnAudioPolicy(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		asset := newAudioAsset(t, tx, f.adminID)
		rejectsWith(t, tx, "questions_audio_policy_iff_audio",
			`INSERT INTO app.questions
			        (type, prompt, points, created_by, media_asset_id, media_asset_kind)
			 VALUES ('short_answer', 'Có audio nhưng thiếu policy', 1.0, $1, $2, 'audio')`,
			f.adminID, asset)
	})
}

func TestAudioQuestionWithItsPolicyIsAccepted(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		asset := newAudioAsset(t, tx, f.adminID)
		mustExec(t, tx,
			`INSERT INTO app.questions
			        (type, prompt, points, created_by, media_asset_id, media_asset_kind,
			         audio_max_plays, audio_allow_seek, audio_show_transcript_after, transcript)
			 VALUES ('short_answer', 'Nghe và trả lời', 2.0, $1, $2, 'audio',
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
			        (type, prompt, points, created_by, media_asset_id, media_asset_kind,
			         audio_allow_seek, audio_show_transcript_after)
			 VALUES ('short_answer', 'Ảnh giả làm audio', 1.0, $1, $2, 'audio', false, true)`,
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
			        (type, prompt, points, created_by, media_asset_id, media_asset_kind,
			         audio_allow_seek, audio_show_transcript_after)
			 VALUES ('short_answer', 'Ảnh có policy audio', 1.0, $1, $2, 'image', false, true)`,
			f.adminID, image)
	})
}

func TestMediaPairMustBeCompleteOrAbsent(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		asset := newAudioAsset(t, tx, f.adminID)
		rejectsWith(t, tx, "questions_media_pair_complete",
			`INSERT INTO app.questions (type, prompt, points, created_by, media_asset_id)
			 VALUES ('short_answer', 'Thiếu kind', 1.0, $1, $2)`, f.adminID, asset)
	})
}

func TestSampleAnswerIsShortAnswerOnly(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "questions_sample_answer_only_short_answer",
			`INSERT INTO app.questions (type, prompt, points, created_by, sample_answer)
			 VALUES ('single_choice', 'Chọn một đáp án', 1.0, $1, 'Đáp án mẫu')`, f.adminID)
	})
}

func TestSampleAnswerOnShortAnswerIsAccepted(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.questions (type, prompt, points, created_by, sample_answer)
			 VALUES ('short_answer', 'Viết câu trả lời', 1.0, $1, 'Đáp án mẫu')`, f.adminID)
	})
}

func TestTranscriptRequiresAudio(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "questions_transcript_iff_audio",
			`INSERT INTO app.questions (type, prompt, points, created_by, transcript)
			 VALUES ('short_answer', 'Không có audio', 1.0, $1, 'Lời thoại')`, f.adminID)
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
			 VALUES ($1, 1, true) RETURNING id`, q).Scan(&blank); err != nil {
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
		`INSERT INTO app.questions (type, prompt, points, created_by)
		 VALUES ($1::app.question_type, $2, 1.0, $3) RETURNING id`,
		qtype, prompt, author).Scan(&id); err != nil {
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
			`INSERT INTO app.questions (type, prompt, points, created_by, audio_max_plays)
			 VALUES ('short_answer', 'Không có audio', 1.0, $1, 3)`, f.adminID)
	})
}

func TestMaxPlaysCannotBeSetOnAnImageQuestion(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		image := newImageAsset(t, tx, f.adminID)
		rejectsWith(t, tx, "questions_max_plays_only_with_audio",
			`INSERT INTO app.questions
			        (type, prompt, points, created_by, media_asset_id, media_asset_kind, audio_max_plays)
			 VALUES ('short_answer', 'Ảnh', 1.0, $1, $2, 'image', 3)`, f.adminID, image)
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
			        (type, prompt, points, created_by, media_asset_id, media_asset_kind,
			         audio_allow_seek, audio_show_transcript_after, audio_max_plays)
			 VALUES ('short_answer', 'Nghe không giới hạn', 1.0, $1, $2, 'audio', false, true, NULL)`,
			f.adminID, asset)
	})
}

func TestAnAudioQuestionMaySetMaxPlays(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		asset := newAudioAsset(t, tx, f.adminID)
		mustExec(t, tx,
			`INSERT INTO app.questions
			        (type, prompt, points, created_by, media_asset_id, media_asset_kind,
			         audio_allow_seek, audio_show_transcript_after, audio_max_plays)
			 VALUES ('short_answer', 'Nghe tối đa 2 lần', 1.0, $1, $2, 'audio', false, true, 2)`,
			f.adminID, asset)
	})
}

// ---------------------------------------------------- tests and drafts

// [D-14] §7's `currentVersion` has to mean something: a published or archived
// test with no snapshot behind it is an assignment pointing at nothing.
func TestAPublishedTestMustHaveAVersion(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		rejectsWith(t, tx, "tests_published_has_version",
			`INSERT INTO app.tests (title, status, current_version, created_by)
			 VALUES ('Đề đã xuất bản', 'published', 0, $1)`, f.adminID)
	})
}

// 00027: only a published test can be pointed at by an assignment, so only
// that status needs a snapshot behind it. An abandoned draft archives as it is.
func TestAnArchivedDraftNeedsNoVersion(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.tests (title, status, current_version, created_by)
			 VALUES ('Đề lưu trữ', 'archived', 0, $1)`, f.adminID)
	})
}

func TestADraftMayHaveNoVersionYet(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.tests (title, created_by) VALUES ('Đề nháp', $1)`, f.adminID)
	})
}

func TestAPublishedTestWithAVersionIsAccepted(t *testing.T) {
	withTx(t, migrated(t), func(tx *sql.Tx, f fixture) {
		mustExec(t, tx,
			`INSERT INTO app.tests (title, status, current_version, created_by)
			 VALUES ('Đề đã xuất bản', 'published', 1, $1)`, f.adminID)
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
			`SELECT test_id::text FROM app.test_sections WHERE id = $1`, section).Scan(&testID); err != nil {
			t.Fatal(err)
		}
		mustExec(t, tx, `DELETE FROM app.tests WHERE id = $1`, testID)

		var sections, links int
		if err := tx.QueryRow(
			`SELECT (SELECT count(*) FROM app.test_sections WHERE test_id = $1),
			        (SELECT count(*) FROM app.test_section_questions WHERE test_section_id = $2)`,
			testID, section).Scan(&sections, &links); err != nil {
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
			`SELECT test_id::text FROM app.test_sections WHERE id = $1`, section).Scan(&testID); err != nil {
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
		`INSERT INTO app.tests (title, created_by) VALUES ('Đề nháp', $1) RETURNING id`,
		author).Scan(&testID); err != nil {
		t.Fatalf("test fixture: %v", err)
	}
	if err := tx.QueryRow(
		`INSERT INTO app.test_sections (test_id, ordinal, title)
		 VALUES ($1, 0, 'Phần 1') RETURNING id`, testID).Scan(&sectionID); err != nil {
		t.Fatalf("section fixture: %v", err)
	}
	return sectionID
}

// ------------------------------------------------- version snapshots

// newVersion creates a published test with one version and one section, and
// returns the section id.
func newVersion(t *testing.T, tx *sql.Tx, author string) string {
	t.Helper()
	var testID, versionID, sectionID string
	if err := tx.QueryRow(
		`INSERT INTO app.tests (title, status, current_version, created_by)
		 VALUES ('Đề đã xuất bản', 'published', 1, $1) RETURNING id`, author).Scan(&testID); err != nil {
		t.Fatalf("test fixture: %v", err)
	}
	if err := tx.QueryRow(
		`INSERT INTO app.test_versions (test_id, version, total_points, published_by)
		 VALUES ($1, 1, 10.00, $2) RETURNING id`, testID, author).Scan(&versionID); err != nil {
		t.Fatalf("version fixture: %v", err)
	}
	if err := tx.QueryRow(
		`INSERT INTO app.test_version_sections (test_version_id, ordinal, title)
		 VALUES ($1, 0, 'Phần 1') RETURNING id`, versionID).Scan(&sectionID); err != nil {
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
			section, q).Scan(&vqID); err != nil {
			t.Fatal(err)
		}

		mustExec(t, tx, `DELETE FROM app.questions WHERE id = $1`, q)

		var source *string
		var prompt string
		var points string
		if err := tx.QueryRow(
			`SELECT source_question_id::text, prompt, points::text
			   FROM app.test_version_questions WHERE id = $1`, vqID).Scan(&source, &prompt, &points); err != nil {
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
			     WHERE conname = 'test_versions_id_test_key' AND contype = 'u')`).Scan(&exists); err != nil {
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
			  WHERE s.id = $1`, section).Scan(&testID); err != nil {
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
			`INSERT INTO app.tests (title, created_by) VALUES ('Đề', $1) RETURNING id`,
			f.adminID).Scan(&testID); err != nil {
			t.Fatal(err)
		}
		rejectsWith(t, tx, "test_versions_version_check",
			`INSERT INTO app.test_versions (test_id, version, total_points, published_by)
			 VALUES ($1, 0, 10.00, $2)`, testID, f.adminID)
	})
}
