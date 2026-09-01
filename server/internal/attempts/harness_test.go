package attempts_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/attempts"
)

// The grading key, in values no student is ever shown. Searched for verbatim
// in the payload, so they have to be unmistakable.
const (
	secretExplanation  = "GIAI-THICH-KHONG-DANH-CHO-HOC-VIEN"
	secretSampleAnswer = "DAP-AN-MAU-KHONG-DANH-CHO-HOC-VIEN"
	secretBlankAnswer  = "DAP-AN-CHO-TRONG-KHONG-DANH-CHO-HOC-VIEN"
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

// world is one open assignment a student may sit: an admin who owns it, a class
// the student is in, and a published version carrying one question of each
// shape that matters to the payload.
type world struct {
	admin      string
	student    string
	outsider   string
	class      string
	testID     string
	versionID  string
	assignment string
	choice     string
	blank      string
	essay      string
}

type worldOpts struct {
	opensAt     time.Time
	closesAt    time.Time
	closedAt    *time.Time
	draft       bool
	maxAttempts int
	duration    int
}

func openAssignment() worldOpts {
	now := time.Now()
	return worldOpts{
		opensAt: now.Add(-time.Hour), closesAt: now.Add(3 * time.Hour),
		maxAttempts: 1, duration: 60,
	}
}

func seedWorld(t *testing.T, pool *pgxpool.Pool, o worldOpts) world {
	t.Helper()
	ctx := context.Background()
	id := nonce(t)
	var w world

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	exec := func(q string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, q, args...)
		must(err)
	}

	for _, u := range []struct {
		into *string
		tag  string
		role string
	}{{&w.admin, "a", "admin"}, {&w.student, "s", "student"}, {&w.outsider, "o", "student"}} {
		must(pool.QueryRow(ctx,
			`INSERT INTO app.users (email, full_name, role)
			 VALUES ($1, 'Người dùng', $2::app.user_role) RETURNING id::text`,
			"att-"+u.tag+"-"+id+"@example.com", u.role).Scan(u.into))
	}

	must(pool.QueryRow(ctx, `INSERT INTO app.classes (name) VALUES ($1) RETURNING id::text`,
		"Lớp "+id).Scan(&w.class))
	exec(`INSERT INTO app.class_members (class_id, user_id, joined_via, added_by)
	      VALUES ($1::uuid,$2::uuid,'admin',$3::uuid)`, w.class, w.student, w.admin)

	must(pool.QueryRow(ctx,
		`INSERT INTO app.tests (title, status, current_version, created_by)
		 VALUES ($1,'published',1,$2::uuid) RETURNING id::text`,
		"Đề "+id, w.admin).Scan(&w.testID))
	must(pool.QueryRow(ctx,
		`INSERT INTO app.test_versions (test_id, version, total_points, published_by)
		 VALUES ($1::uuid,1,'10.00',$2::uuid) RETURNING id::text`,
		w.testID, w.admin).Scan(&w.versionID))

	var section string
	must(pool.QueryRow(ctx,
		`INSERT INTO app.test_version_sections (test_version_id, ordinal, title)
		 VALUES ($1::uuid,0,'Phần 1') RETURNING id::text`, w.versionID).Scan(&section))

	// Every leaky column gets a value here on purpose, and a distinctive one:
	// a projection that selected them would be caught by a test rather than by
	// a student. The sentinels are searched for by value, so they must not
	// occur anywhere a student is legitimately shown text.
	must(pool.QueryRow(ctx, `
		INSERT INTO app.test_version_questions
		  (test_version_section_id, ordinal, type, prompt, points, explanation, sample_answer)
		VALUES ($1::uuid,0,'single_choice','Thủ đô của Việt Nam?','5.00',$2,$3)
		RETURNING id::text`, section, secretExplanation, secretSampleAnswer).Scan(&w.choice))
	for i, opt := range []struct {
		text    string
		correct bool
	}{{"Hà Nội", true}, {"Huế", false}, {"Đà Nẵng", false}, {"Cần Thơ", false}} {
		exec(`INSERT INTO app.test_version_options (test_version_question_id, ordinal, text, is_correct)
		      VALUES ($1::uuid,$2,$3,$4)`, w.choice, i, opt.text, opt.correct)
	}

	must(pool.QueryRow(ctx, `
		INSERT INTO app.test_version_questions
		  (test_version_section_id, ordinal, type, prompt, points)
		VALUES ($1::uuid,1,'fill_blank','She {{1}} in Hanoi since {{2}}.','5.00')
		RETURNING id::text`, section).Scan(&w.blank))
	for ordinal := 1; ordinal <= 2; ordinal++ {
		var blankID string
		must(pool.QueryRow(ctx,
			`INSERT INTO app.test_version_blanks (test_version_question_id, ordinal, case_sensitive)
			 VALUES ($1::uuid,$2,true) RETURNING id::text`, w.blank, ordinal).Scan(&blankID))
		exec(`INSERT INTO app.test_version_blank_answers (test_version_blank_id, answer)
		      VALUES ($1::uuid,$2)`, blankID, secretBlankAnswer)
	}

	// The one type §7 grades by hand, so requires_manual has something to be
	// true about.
	must(pool.QueryRow(ctx, `
		INSERT INTO app.test_version_questions
		  (test_version_section_id, ordinal, type, prompt, points, sample_answer)
		VALUES ($1::uuid,2,'short_answer','Viết 2-3 câu tả thói quen buổi sáng.','5.00',$2)
		RETURNING id::text`, section, secretSampleAnswer).Scan(&w.essay))

	published := "now()"
	if o.draft {
		published = "NULL"
	}
	must(pool.QueryRow(ctx, `
		INSERT INTO app.assignments
		  (test_id, test_version_id, opens_at, closes_at, closed_at, published_at,
		   duration_minutes, max_attempts, created_by)
		VALUES ($1::uuid,$2::uuid,$3,$4,$5,`+published+`,$6,$7,$8::uuid)
		RETURNING id::text`,
		w.testID, w.versionID, o.opensAt, o.closesAt, o.closedAt,
		o.duration, o.maxAttempts, w.admin).Scan(&w.assignment))
	exec(`INSERT INTO app.assignment_classes (assignment_id, class_id)
	      VALUES ($1::uuid,$2::uuid)`, w.assignment, w.class)

	t.Cleanup(func() {
		c := context.Background()
		for _, q := range []string{
			`DELETE FROM app.attempt_events WHERE attempt_id IN
			   (SELECT id FROM app.attempts WHERE assignment_id = $1::uuid)`,
			`DELETE FROM app.attempt_answers WHERE attempt_id IN
			   (SELECT id FROM app.attempts WHERE assignment_id = $1::uuid)`,
			`DELETE FROM app.attempts WHERE assignment_id = $1::uuid`,
			`DELETE FROM app.assignment_classes WHERE assignment_id = $1::uuid`,
			`DELETE FROM app.assignment_students WHERE assignment_id = $1::uuid`,
			`DELETE FROM app.assignments WHERE id = $1::uuid`,
		} {
			_, _ = pool.Exec(c, q, w.assignment)
		}
		_, _ = pool.Exec(c, `DELETE FROM app.test_versions WHERE test_id = $1::uuid`, w.testID)
		_, _ = pool.Exec(c, `DELETE FROM app.tests WHERE id = $1::uuid`, w.testID)
		_, _ = pool.Exec(c, `DELETE FROM app.class_members WHERE class_id = $1::uuid`, w.class)
		_, _ = pool.Exec(c, `DELETE FROM app.classes WHERE id = $1::uuid`, w.class)
		_, _ = pool.Exec(c, `DELETE FROM app.users WHERE id IN ($1::uuid,$2::uuid,$3::uuid)`,
			w.admin, w.student, w.outsider)
	})
	return w
}

func newService(t *testing.T, pool *pgxpool.Pool) *attempts.Service {
	t.Helper()
	return attempts.NewService(attempts.NewStore(pool))
}
