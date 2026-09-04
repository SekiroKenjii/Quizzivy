package integrity_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/integrity"
)

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedAttempt is one attempt started at a known moment, with one listening
// question allowing two plays that the student played three times.
func seedAttempt(t *testing.T, pool *pgxpool.Pool, startedAt time.Time) (attemptID, questionID string) {
	t.Helper()
	ctx := context.Background()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	id := hex.EncodeToString(b)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	var admin, student, testID, versionID, sectionID, assetID, assignmentID string
	must(pool.QueryRow(ctx, `INSERT INTO app.users (email, full_name, role) VALUES ($1,'Giáo viên','admin') RETURNING id::text`, "int-a-"+id+"@example.com").Scan(&admin))
	must(pool.QueryRow(ctx, `INSERT INTO app.users (email, full_name, role) VALUES ($1,'Học viên','student') RETURNING id::text`, "int-s-"+id+"@example.com").Scan(&student))
	// Registered before the rest so a fixture that fails half-way still leaves nothing behind.
	t.Cleanup(func() {
		c := context.Background()
		for _, step := range []struct {
			q   string
			arg string
		}{
			{`DELETE FROM app.attempt_audio_plays WHERE attempt_id IN (SELECT id FROM app.attempts WHERE student_id = $1::uuid)`, student},
			{`DELETE FROM app.attempt_events WHERE attempt_id IN (SELECT id FROM app.attempts WHERE student_id = $1::uuid)`, student},
			{`DELETE FROM app.attempts WHERE student_id = $1::uuid`, student},
			{`DELETE FROM app.assignment_classes WHERE assignment_id IN (SELECT id FROM app.assignments WHERE created_by = $1::uuid)`, admin},
			{`DELETE FROM app.assignments WHERE created_by = $1::uuid`, admin},
			{`DELETE FROM app.test_versions WHERE test_id IN (SELECT id FROM app.tests WHERE created_by = $1::uuid)`, admin},
			{`DELETE FROM app.media_assets WHERE uploaded_by = $1::uuid`, admin},
			{`DELETE FROM app.tests WHERE created_by = $1::uuid`, admin},
			{`DELETE FROM app.users WHERE id = $1::uuid`, student},
			{`DELETE FROM app.users WHERE id = $1::uuid`, admin},
		} {
			if _, err := pool.Exec(c, step.q, step.arg); err != nil {
				t.Logf("cleanup: %v", err)
			}
		}
	})
	must(pool.QueryRow(ctx, `INSERT INTO app.tests (title, status, current_version, created_by) VALUES ('Integrity', 'published', 1, $1::uuid) RETURNING id::text`, admin).Scan(&testID))
	must(pool.QueryRow(ctx, `INSERT INTO app.test_versions (test_id, version, total_points, published_by) VALUES ($1::uuid, 1, '5.00', $2::uuid) RETURNING id::text`, testID, admin).Scan(&versionID))
	must(pool.QueryRow(ctx, `INSERT INTO app.test_version_sections (test_version_id, ordinal, title) VALUES ($1::uuid, 0, 'Phần 1') RETURNING id::text`, versionID).Scan(&sectionID))
	must(pool.QueryRow(ctx, `INSERT INTO app.media_assets (kind, storage_key, mime_type, bytes, duration_ms, original_filename, checksum_sha256, uploaded_by)
		VALUES ('audio', $1, 'audio/mpeg', 1000, 10000, 'a.mp3', sha256(convert_to($1, 'UTF8')), $2::uuid) RETURNING id::text`, "audio/int-"+id+".mp3", admin).Scan(&assetID))
	must(pool.QueryRow(ctx, `INSERT INTO app.test_version_questions
		(test_version_section_id, ordinal, type, prompt, points, media_asset_id, media_asset_kind, audio_max_plays, audio_allow_seek, audio_show_transcript_after)
		VALUES ($1::uuid, 0, 'single_choice', 'Nghe', '5.00', $2::uuid, 'audio', 2, false, false) RETURNING id::text`, sectionID, assetID).Scan(&questionID))
	must(pool.QueryRow(ctx, `INSERT INTO app.assignments (test_id, test_version_id, opens_at, closes_at, duration_minutes, created_by, published_at, integrity_min_away_ms)
		VALUES ($1::uuid, $2::uuid, now() - interval '2 hours', now() + interval '2 hours', 45, $3::uuid, now(), 3000) RETURNING id::text`, testID, versionID, admin).Scan(&assignmentID))
	must(pool.QueryRow(ctx, `INSERT INTO app.attempts (assignment_id, test_version_id, student_id, attempt_no, session_id, shuffle_seed, beacon_token_hash, started_at, deadline_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 1, gen_random_uuid(), 1, sha256('b'::bytea), $4::timestamptz, $4::timestamptz + interval '45 minutes') RETURNING id::text`, assignmentID, versionID, student, startedAt).Scan(&attemptID))
	// Two options, so the seed's frozen-question invariant holds even mid-run.
	for i, text := range []string{"Đi bộ", "Đi xe buýt"} {
		_, err := pool.Exec(ctx, `INSERT INTO app.test_version_options (test_version_question_id, ordinal, text, is_correct)
			VALUES ($1::uuid, $2, $3, $4)`, questionID, i, text, i == 0)
		must(err)
	}
	_, err := pool.Exec(ctx, `INSERT INTO app.attempt_audio_plays (attempt_id, question_id, plays) VALUES ($1::uuid, $2::uuid, 3)`, attemptID, questionID)
	must(err)
	return attemptID, questionID
}

func TestTheTimelineReadsTheLogAcrossAResumeAndTheReplaysFromThePlaysTable(t *testing.T) {
	pool := newPool(t)
	startedAt := time.Now().Add(-30 * time.Minute).Truncate(time.Millisecond)
	attemptID, questionID := seedAttempt(t, pool, startedAt)
	ctx := context.Background()

	s1, s2 := "0195a000-0000-7000-8000-000000000001", "0195a000-0000-7000-8000-000000000002"
	at := func(seconds int) time.Time { return startedAt.Add(time.Duration(seconds) * time.Second) }
	for _, e := range []struct {
		session  string
		kind     string
		seconds  int
		seq      *int
		question *string
	}{
		{s1, "window_blur", 60, ptr(0), nil},
		{s1, "tab_hidden", 60, ptr(1), nil},
		{s1, "audio_play", 100, ptr(2), &questionID},
		{s1, "session_takeover", 200, nil, nil},
		{s2, "resume", 200, nil, nil},
		{s2, "window_focus", 201, ptr(0), nil},
		{s2, "paste", 260, ptr(1), &questionID},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO app.attempt_events (attempt_id, session_id, kind, occurred_at, received_at, client_seq, question_id)
			VALUES ($1::uuid, $2::uuid, $3, $4, $4, $5, $6::uuid)`, attemptID, e.session, e.kind, at(e.seconds), e.seq, e.question); err != nil {
			t.Fatal(err)
		}
	}

	timeline, err := integrity.NewStore(pool).Timeline(ctx, attemptID)
	if err != nil {
		t.Fatalf("timeline: %v", err)
	}
	if !timeline.StartedAt.Equal(startedAt) {
		t.Errorf("startedAt %v, want %v", timeline.StartedAt, startedAt)
	}
	if len(timeline.Events) != 7 {
		t.Fatalf("%d events, want 7", len(timeline.Events))
	}
	first := timeline.Events[0]
	if first.Kind != "window_blur" || first.DurationMs == nil || *first.DurationMs != 141000 {
		t.Errorf("the leave before the takeover should close on the focus after the resume: %+v", first)
	}
	if first.OffsetMs != 60000 {
		t.Errorf("offset %d, want 60000", first.OffsetMs)
	}
	want := integrity.Summary{TotalAwayMs: 141000, AwayEpisodes: 1, PasteCount: 1, ResumeCount: 1, AudioReplays: 1}
	if timeline.Summary != want {
		t.Errorf("summary %+v, want %+v", timeline.Summary, want)
	}

	if _, err := integrity.NewStore(pool).Timeline(ctx, "01935000-0000-7000-8000-00000000dead"); err != integrity.ErrNotFound {
		t.Errorf("unknown attempt: %v, want ErrNotFound", err)
	}
}

func ptr(n int) *int { return &n }
