package media_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/media"
)

// §11.2: a student may mint a signed URL only for audio in a test they are
// actually sitting. The asset id comes from the client, so this is the check
// that stops one student reading another class's listening files by guessing.

func nonce(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

// reachabilityFixture builds a published version holding one audio question,
// and a second asset that no version references at all.
type reachabilityFixture struct {
	sitter     string // has an attempt on the version
	outsider   string // has no attempt
	usedAsset  string
	looseAsset string
}

func seedReachability(t *testing.T, pool *pgxpool.Pool) reachabilityFixture {
	t.Helper()
	ctx := context.Background()
	id := nonce(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	var author, sitter, outsider string
	must(pool.QueryRow(ctx, `INSERT INTO app.users (email, full_name, role)
		VALUES ($1,'Giáo viên','admin') RETURNING id::text`, "reach-a-"+id+"@example.com").Scan(&author))
	must(pool.QueryRow(ctx, `INSERT INTO app.users (email, full_name, role)
		VALUES ($1,'Người học','student') RETURNING id::text`, "reach-s-"+id+"@example.com").Scan(&sitter))
	must(pool.QueryRow(ctx, `INSERT INTO app.users (email, full_name, role)
		VALUES ($1,'Người ngoài','student') RETURNING id::text`, "reach-o-"+id+"@example.com").Scan(&outsider))

	var used, loose string
	for _, target := range []*string{&used, &loose} {
		must(pool.QueryRow(ctx, `
			INSERT INTO app.media_assets
			       (kind, storage_key, mime_type, bytes, duration_ms, original_filename,
			        checksum_sha256, uploaded_by)
			VALUES ('audio', $1, 'audio/mpeg', 1024, 10000, 'nghe.mp3',
			        sha256(convert_to($1, 'UTF8')), $2)
			RETURNING id::text`, "reach/"+id+"-"+nonce(t)+".mp3", author).Scan(target))
	}

	var testID, versionID, sectionID, assignmentID string
	must(pool.QueryRow(ctx, `INSERT INTO app.tests (title, status, current_version, created_by)
		VALUES ('Reachability','published',1,$1) RETURNING id::text`, author).Scan(&testID))
	must(pool.QueryRow(ctx, `INSERT INTO app.test_versions (test_id, version, total_points, published_by)
		VALUES ($1,1,'2.00',$2) RETURNING id::text`, testID, author).Scan(&versionID))
	must(pool.QueryRow(ctx, `INSERT INTO app.test_version_sections (test_version_id, ordinal, title)
		VALUES ($1,0,'Nghe') RETURNING id::text`, versionID).Scan(&sectionID))
	must(func() error {
		_, err := pool.Exec(ctx, `
			INSERT INTO app.test_version_questions
			       (test_version_section_id, ordinal, type, prompt, points,
			        media_asset_id, media_asset_kind,
			        audio_max_plays, audio_allow_seek, audio_show_transcript_after)
			VALUES ($1,0,'single_choice','Nghe và chọn','2.00',$2,'audio',2,false,true)`,
			sectionID, used)
		return err
	}())
	must(pool.QueryRow(ctx, `INSERT INTO app.assignments
		       (test_id, test_version_id, opens_at, closes_at, duration_minutes, created_by)
		VALUES ($1,$2, now() - interval '1 hour', now() + interval '1 hour', 45, $3)
		RETURNING id::text`, testID, versionID, author).Scan(&assignmentID))

	var attemptID string
	must(pool.QueryRow(ctx, `INSERT INTO app.attempts
		       (assignment_id, test_version_id, student_id, attempt_no, status, session_id,
		        shuffle_seed, beacon_token_hash, deadline_at)
		VALUES ($1,$2,$3,1,'in_progress', gen_random_uuid(), 7, sha256('b'::bytea),
		        now() + interval '1 hour')
		RETURNING id::text`, assignmentID, versionID, sitter).Scan(&attemptID))

	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM app.attempts WHERE id = $1`, attemptID)
		_, _ = pool.Exec(c, `DELETE FROM app.assignments WHERE id = $1`, assignmentID)
		_, _ = pool.Exec(c, `DELETE FROM app.test_version_questions WHERE test_version_section_id = $1`, sectionID)
		_, _ = pool.Exec(c, `DELETE FROM app.test_version_sections WHERE id = $1`, sectionID)
		_, _ = pool.Exec(c, `DELETE FROM app.test_versions WHERE id = $1`, versionID)
		_, _ = pool.Exec(c, `DELETE FROM app.tests WHERE id = $1`, testID)
		_, _ = pool.Exec(c, `DELETE FROM app.media_assets WHERE id IN ($1,$2)`, used, loose)
		_, _ = pool.Exec(c, `DELETE FROM app.audit_log WHERE actor_user_id IN ($1,$2,$3)`, author, sitter, outsider)
		_, _ = pool.Exec(c, `DELETE FROM app.users WHERE id IN ($1,$2,$3)`, author, sitter, outsider)
	})

	return reachabilityFixture{sitter: sitter, outsider: outsider, usedAsset: used, looseAsset: loose}
}

func TestAStudentSittingTheTestReachesItsAudio(t *testing.T) {
	pool := newPool(t)
	f := seedReachability(t, pool)

	ok, err := media.ReachableByStudent(context.Background(), pool, f.sitter, f.usedAsset)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("the student has an attempt on the version that uses this asset; they must reach it")
	}
}

func TestAStudentWithoutAnAttemptReachesNothing(t *testing.T) {
	pool := newPool(t)
	f := seedReachability(t, pool)

	// The asset id is supplied by the client, so this is the guess that must fail.
	ok, err := media.ReachableByStudent(context.Background(), pool, f.outsider, f.usedAsset)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("a student with no attempt on the version reached its audio")
	}
}

func TestAnAssetNoVersionUsesIsNotReachable(t *testing.T) {
	pool := newPool(t)
	f := seedReachability(t, pool)

	ok, err := media.ReachableByStudent(context.Background(), pool, f.sitter, f.looseAsset)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("an asset attached to no question was reachable")
	}
}
