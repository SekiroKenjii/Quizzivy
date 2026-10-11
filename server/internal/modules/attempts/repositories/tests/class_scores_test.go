//go:build integration

package repositories_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/attempts/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/stats"
)

type scoresWorld struct {
	t        *testing.T
	pool     *pgxpool.Pool
	marker   string
	teacher  string
	version  string
	section  string
	question string
}

func newScoresWorld(t *testing.T) *scoresWorld {
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
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	w := &scoresWorld{t: t, pool: pool, marker: "Điểm lớp " + hex.EncodeToString(raw)}
	t.Cleanup(func() {
		ctx := context.Background()
		users := `(SELECT id FROM app.users WHERE full_name LIKE '%' || $1)`
		classes := `(SELECT id FROM app.classes WHERE name LIKE '%' || $1)`
		tests := `(SELECT id FROM app.tests WHERE title LIKE '%' || $1)`
		versions := `(SELECT id FROM app.test_versions WHERE test_id IN ` + tests + `)`
		sections := `(SELECT id FROM app.test_version_sections WHERE test_version_id IN ` + versions + `)`
		for _, stmt := range []string{
			`DELETE FROM app.attempt_answers WHERE attempt_id IN (SELECT id FROM app.attempts WHERE student_id IN ` + users + `)`,
			`DELETE FROM app.attempts WHERE student_id IN ` + users,
			`DELETE FROM app.assignment_classes WHERE class_id IN ` + classes,
			`DELETE FROM app.assignments WHERE test_id IN ` + tests,
			`DELETE FROM app.test_version_questions WHERE test_version_section_id IN ` + sections,
			`DELETE FROM app.test_version_sections WHERE test_version_id IN ` + versions,
			`DELETE FROM app.test_versions WHERE test_id IN ` + tests,
			`DELETE FROM app.tests WHERE title LIKE '%' || $1`,
			`DELETE FROM app.class_members WHERE class_id IN ` + classes,
			`DELETE FROM app.classes WHERE name LIKE '%' || $1`,
			`DELETE FROM app.users WHERE full_name LIKE '%' || $1`,
		} {
			if _, err := pool.Exec(ctx, stmt, w.marker); err != nil {
				t.Errorf("cleanup %q: %v", stmt, err)
			}
		}
	})
	w.teacher = w.id(`INSERT INTO app.users (email, full_name, role_id) VALUES ($1, $2, (SELECT id FROM app.roles WHERE builtin_key = 'teacher')) RETURNING id::text`,
		uuid.NewString()+"@example.test", "Giáo viên "+w.marker)
	test := w.id(`INSERT INTO app.tests (title, status, current_version, created_by, owner_id) VALUES ($1, 'published', 1, $2, $2) RETURNING id::text`, "Đề "+w.marker, w.teacher)
	w.version = w.id(`INSERT INTO app.test_versions (test_id, version, total_points, published_by) VALUES ($1, 1, 10, $2) RETURNING id::text`, test, w.teacher)
	w.section = w.id(`INSERT INTO app.test_version_sections (test_version_id, ordinal, title) VALUES ($1, 0, 'Phần 1') RETURNING id::text`, w.version)
	w.question = w.id(`INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, points) VALUES ($1, 0, 'short_answer', 'Câu hỏi', 1) RETURNING id::text`, w.section)
	return w
}

func (w *scoresWorld) id(sql string, args ...any) string {
	w.t.Helper()
	var id string
	if err := w.pool.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		w.t.Fatal(err)
	}
	return id
}

func (w *scoresWorld) exec(sql string, args ...any) {
	w.t.Helper()
	if _, err := w.pool.Exec(context.Background(), sql, args...); err != nil {
		w.t.Fatal(err)
	}
}

func (w *scoresWorld) student(name string) string {
	return w.id(`INSERT INTO app.users (email, full_name, role_id) VALUES ($1, $2, (SELECT id FROM app.roles WHERE builtin_key = 'student')) RETURNING id::text`,
		uuid.NewString()+"@example.test", name+" "+w.marker)
}

func (w *scoresWorld) class(name string) string {
	return w.id(`INSERT INTO app.classes (name, teacher_id) VALUES ($1, $2) RETURNING id::text`, name+" "+w.marker, w.teacher)
}

func (w *scoresWorld) enrol(class, student string) {
	w.exec(`INSERT INTO app.class_members (class_id, user_id, joined_via, added_by) VALUES ($1, $2, 'admin', $3)`, class, student, w.teacher)
}

func (w *scoresWorld) assignment(class string) string {
	id := w.id(`INSERT INTO app.assignments (test_id, test_version_id, opens_at, closes_at, duration_minutes, created_by, published_at)
		SELECT v.test_id, v.id, now() - interval '30 days', now() + interval '2 hours', 45, $2, now() FROM app.test_versions v WHERE v.id = $1 RETURNING id::text`,
		w.version, w.teacher)
	w.exec(`INSERT INTO app.assignment_classes (assignment_id, class_id) VALUES ($1, $2)`, id, class)
	return id
}

func (w *scoresWorld) attempt(assignment, student string, no int, status string, earned, total *float64) string {
	return w.id(`
		INSERT INTO app.attempts (assignment_id, test_version_id, student_id, attempt_no, status, session_id, shuffle_seed, beacon_token_hash,
		                          started_at, deadline_at, submitted_at, graded_at, score_earned, score_total, void_reason)
		VALUES ($1, $2, $3, $4, $5::app.attempt_status, gen_random_uuid(), 1, sha256('b'::bytea),
		        now() - interval '2 hours', now() - interval '1 hour',
		        CASE WHEN $5 <> 'in_progress' THEN now() - interval '90 minutes' END,
		        CASE WHEN $5 = 'graded' THEN now() - interval '80 minutes' END,
		        $6, $7, CASE WHEN $5 = 'voided' THEN 'Không hợp lệ' END)
		RETURNING id::text`, assignment, w.version, student, no, status, earned, total)
}

func (w *scoresWorld) unmarked(attempt string) {
	w.exec(`INSERT INTO app.attempt_answers (attempt_id, question_id, payload, requires_manual) VALUES ($1, $2, '{"type":"text","value":"Bài làm"}', true)`, attempt, w.question)
}

func points(v float64) *float64 { return &v }

func TestAClassScoreIsTheBestGradedAttemptOfEachLiveMemberOnEachOfItsAssignments(t *testing.T) {
	w := newScoresWorld(t)
	classK, classL, classM := w.class("K"), w.class("L"), w.class("M")
	a, b, c, left, disabled, other := w.student("A"), w.student("B"), w.student("C"), w.student("D"), w.student("E"), w.student("F")
	for _, student := range []string{a, b, c, left, disabled} {
		w.enrol(classK, student)
	}
	for _, student := range []string{a, other} {
		w.enrol(classL, student)
	}
	w.exec(`UPDATE app.users SET disabled_at = now() WHERE id = $1`, disabled)
	x, y, z := w.assignment(classK), w.assignment(classK), w.assignment(classL)

	w.unmarked(w.attempt(x, a, 1, "graded", points(5), points(10)))
	best := w.attempt(x, a, 2, "graded", points(8), points(10))
	w.unmarked(best)
	w.attempt(x, a, 3, "voided", points(10), points(10))
	w.attempt(x, b, 1, "submitted", nil, nil)
	w.attempt(x, b, 2, "in_progress", nil, nil)
	w.attempt(y, b, 1, "graded", points(6), nil)
	w.attempt(y, c, 1, "graded", points(10), points(20))
	w.attempt(y, c, 2, "graded", points(5), points(10))
	w.attempt(x, left, 1, "graded", points(10), points(10))
	w.attempt(y, disabled, 1, "graded", points(10), points(10))
	w.attempt(z, a, 1, "graded", points(9), points(10))
	w.attempt(z, other, 1, "graded", points(7), points(10))
	w.exec(`DELETE FROM app.class_members WHERE class_id = $1 AND user_id = $2`, classK, left)
	w.enrol(classM, left)

	source := repositories.NewStudentStats(db.NewContext(w.pool))
	scores, err := source.ClassScores(context.Background(), []string{classK, classL, classM, uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}

	if got, want := scores[classK], (stats.ClassScore{Earned: 19, Total: 30, PendingManual: 1}); got != want {
		t.Errorf("K scored %+v, want %+v: A's best of three (the voided 10/10 and the 5/10 do not count, the unmarked answer of the best does), B's 6 of the version's 10 (the submitted and the live attempt do not count), C's later attempt on a tie, and nobody who left or is disabled", got, want)
	}
	if got, want := scores[classL], (stats.ClassScore{Earned: 16, Total: 20}); got != want {
		t.Errorf("L scored %+v, want %+v: only the assignment that targets L, for both of its members", got, want)
	}
	if got, ok := scores[classM]; ok {
		t.Errorf("a class with nothing graded scored %+v, want none", got)
	}
	if len(scores) != 2 {
		t.Errorf("%d classes scored, want 2", len(scores))
	}
	if none, err := source.ClassScores(context.Background(), nil); err != nil || len(none) != 0 {
		t.Errorf("no classes asked: %v (%v)", none, err)
	}
}
