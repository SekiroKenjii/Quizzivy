package review_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
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

func nonce(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

// paper is one submitted attempt: a choice question the machine scored right
// and an essay nobody has read yet, out of 10.
type paper struct {
	admin, student string
	testID         string
	attempt        string
	choice, essay  string
}

func seedPaper(t *testing.T, pool *pgxpool.Pool, status string) paper {
	t.Helper()
	ctx := context.Background()
	id := nonce(t)
	var p paper
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	must(pool.QueryRow(ctx, `INSERT INTO app.users (email, full_name, role) VALUES ($1,'Giáo viên','admin') RETURNING id::text`,
		"rev-a-"+id+"@example.com").Scan(&p.admin))
	must(pool.QueryRow(ctx, `INSERT INTO app.users (email, full_name, role) VALUES ($1,'Nguyễn Đức Minh','student') RETURNING id::text`,
		"rev-s-"+id+"@example.com").Scan(&p.student))

	var versionID, sectionID, assignmentID string
	must(pool.QueryRow(ctx, `INSERT INTO app.tests (title, status, current_version, created_by)
		VALUES ('Unit 5', 'published', 1, $1::uuid) RETURNING id::text`, p.admin).Scan(&p.testID))
	must(pool.QueryRow(ctx, `INSERT INTO app.test_versions (test_id, version, total_points, published_by)
		VALUES ($1::uuid, 1, '10.00', $2::uuid) RETURNING id::text`, p.testID, p.admin).Scan(&versionID))
	must(pool.QueryRow(ctx, `INSERT INTO app.test_version_sections (test_version_id, ordinal, title)
		VALUES ($1::uuid, 0, 'Phần 1') RETURNING id::text`, versionID).Scan(&sectionID))
	must(pool.QueryRow(ctx, `INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, points, explanation)
		VALUES ($1::uuid, 0, 'single_choice', 'Thủ đô?', '5.00', 'Vì thế') RETURNING id::text`, sectionID).Scan(&p.choice))
	for i, o := range []struct {
		text    string
		correct bool
	}{{"Hà Nội", true}, {"Huế", false}} {
		_, err := pool.Exec(ctx, `INSERT INTO app.test_version_options (test_version_question_id, ordinal, text, is_correct)
			VALUES ($1::uuid, $2, $3, $4)`, p.choice, i, o.text, o.correct)
		must(err)
	}
	must(pool.QueryRow(ctx, `INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, points, sample_answer)
		VALUES ($1::uuid, 1, 'short_answer', 'Tả buổi sáng', '5.00', 'I get up at six.') RETURNING id::text`, sectionID).Scan(&p.essay))

	must(pool.QueryRow(ctx, `INSERT INTO app.assignments
		(test_id, test_version_id, opens_at, closes_at, duration_minutes, max_attempts, created_by, published_at)
		VALUES ($1::uuid, $2::uuid, now() - interval '2 hours', now() + interval '2 hours', 45, 2, $3::uuid, now())
		RETURNING id::text`, p.testID, versionID, p.admin).Scan(&assignmentID))

	submitted := "now()"
	if status == "in_progress" {
		submitted = "NULL"
	}
	must(pool.QueryRow(ctx, `INSERT INTO app.attempts
		(assignment_id, test_version_id, student_id, attempt_no, status, session_id, shuffle_seed,
		 beacon_token_hash, started_at, deadline_at, submitted_at, score_earned, score_total, void_reason)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 1, $4::app.attempt_status, gen_random_uuid(), 1,
		        sha256('b'::bytea), now() - interval '40 minutes', now() + interval '5 minutes', `+submitted+`,
		        CASE WHEN $4 = 'in_progress' THEN NULL ELSE 5.00 END,
		        CASE WHEN $4 = 'in_progress' THEN NULL ELSE 10.00 END,
		        CASE WHEN $4 = 'voided' THEN 'test' END)
		RETURNING id::text`, assignmentID, versionID, p.student, status).Scan(&p.attempt))

	var correct string
	must(pool.QueryRow(ctx, `SELECT id::text FROM app.test_version_options WHERE test_version_question_id = $1::uuid AND is_correct`, p.choice).Scan(&correct))
	_, err := pool.Exec(ctx, `INSERT INTO app.attempt_answers (attempt_id, question_id, payload, requires_manual, auto_score)
		VALUES ($1::uuid, $2::uuid, $3::jsonb, false, 5.00)`, p.attempt, p.choice, `{"type":"choice","optionIds":["`+correct+`"]}`)
	must(err)
	_, err = pool.Exec(ctx, `INSERT INTO app.attempt_answers (attempt_id, question_id, payload, requires_manual)
		VALUES ($1::uuid, $2::uuid, '{"type":"text","value":"I usually wake up at six."}'::jsonb, true)`, p.attempt, p.essay)
	must(err)

	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM app.attempt_answers WHERE attempt_id = $1::uuid`, p.attempt)
		_, _ = pool.Exec(c, `DELETE FROM app.attempt_events WHERE attempt_id = $1::uuid`, p.attempt)
		_, _ = pool.Exec(c, `DELETE FROM app.attempts WHERE id = $1::uuid`, p.attempt)
		_, _ = pool.Exec(c, `DELETE FROM app.assignments WHERE id = $1::uuid`, assignmentID)
		_, _ = pool.Exec(c, `DELETE FROM app.test_versions WHERE test_id = $1::uuid`, p.testID)
		_, _ = pool.Exec(c, `DELETE FROM app.tests WHERE id = $1::uuid`, p.testID)
		_, _ = pool.Exec(c, `DELETE FROM app.users WHERE id IN ($1::uuid, $2::uuid)`, p.admin, p.student)
	})
	return p
}
