//go:build integration

package repositories_test

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/modules/notifications/repositories"
	"quizzivy/internal/platform/db"
)

var instant = time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)

type dueWorld struct {
	t       *testing.T
	tx      pgx.Tx
	store   *repositories.Postgres
	now     time.Time
	creator string
	class   string
	student string
}

type spec struct {
	title       string
	creator     string
	classes     []string
	students    []string
	opens       time.Time
	closes      time.Time
	published   *time.Time
	draft       bool
	closedAt    *time.Time
	maxAttempts int
	release     string
	hideScore   bool
}

func newDueWorld(t *testing.T) *dueWorld {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tx.Rollback(context.Background()); err != nil {
			t.Errorf("rolling the world back: %v", err)
		}
	})
	w := &dueWorld{t: t, tx: tx, store: repositories.NewPostgres(db.NewContext(tx)), now: instant}
	w.creator = w.user("teacher")
	w.class = w.classOf(w.creator)
	w.student = w.user("student")
	w.join(w.class, w.student)
	return w
}

func (w *dueWorld) id(sql string, args ...any) string {
	w.t.Helper()
	var id string
	if err := w.tx.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		w.t.Fatal(err)
	}
	return id
}

func (w *dueWorld) exec(sql string, args ...any) {
	w.t.Helper()
	if _, err := w.tx.Exec(context.Background(), sql, args...); err != nil {
		w.t.Fatal(err)
	}
}

func (w *dueWorld) user(builtin string) string {
	w.t.Helper()
	return w.id(`INSERT INTO app.users (email, full_name, role_id)
		VALUES ($1, 'Thành viên', (SELECT id FROM app.roles WHERE builtin_key = $2)) RETURNING id::text`,
		uuid.NewString()+"@example.test", builtin)
}

func (w *dueWorld) disabled(userID string) {
	w.t.Helper()
	w.exec(`UPDATE app.users SET disabled_at = $2 WHERE id = $1::uuid`, userID, w.now)
}

func (w *dueWorld) classOf(teacher string) string {
	w.t.Helper()
	return w.id(`INSERT INTO app.classes (name, teacher_id) VALUES ('Lớp thử', $1) RETURNING id::text`, teacher)
}

func (w *dueWorld) join(class, user string) {
	w.t.Helper()
	w.exec(`INSERT INTO app.class_members (class_id, user_id, joined_via, added_by) VALUES ($1, $2, 'admin', $2)`, class, user)
}

func (w *dueWorld) prefer(userID string, event domain.Event, inApp bool) {
	w.t.Helper()
	w.exec(`INSERT INTO app.notification_preferences (user_id, event, in_app) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, event) DO UPDATE SET in_app = EXCLUDED.in_app`, userID, string(event), inApp)
}

func (w *dueWorld) assignment(s spec) string {
	w.t.Helper()
	if s.creator == "" {
		s.creator = w.creator
	}
	if s.classes == nil && s.students == nil {
		s.classes = []string{w.class}
	}
	if s.title == "" {
		s.title = "Đề kiểm tra Unit 5"
	}
	if s.maxAttempts == 0 {
		s.maxAttempts = 1
	}
	if s.release == "" {
		s.release = "on_submit"
	}
	published := s.opens.Add(-time.Hour)
	if s.published != nil {
		published = *s.published
	}
	var publishedAt *time.Time
	if !s.draft {
		publishedAt = &published
	}
	test := w.id(`INSERT INTO app.tests (title, status, current_version, created_by, owner_id)
		VALUES ($1, 'published', 1, $2, $2) RETURNING id::text`, s.title, s.creator)
	version := w.id(`INSERT INTO app.test_versions (test_id, version, total_points, published_by)
		VALUES ($1, 1, 10, $2) RETURNING id::text`, test, s.creator)
	assignment := w.id(`INSERT INTO app.assignments
		(test_id, test_version_id, opens_at, closes_at, closed_at, duration_minutes, max_attempts, created_by,
		 published_at, review_release, review_show_score)
		VALUES ($1, $2, $3, $4, $5, 45, $6, $7, $8, $9, $10) RETURNING id::text`,
		test, version, s.opens, s.closes, s.closedAt, s.maxAttempts, s.creator, publishedAt, s.release, !s.hideScore)
	for _, class := range s.classes {
		w.exec(`INSERT INTO app.assignment_classes (assignment_id, class_id) VALUES ($1, $2)`, assignment, class)
	}
	for _, student := range s.students {
		w.exec(`INSERT INTO app.assignment_students (assignment_id, user_id) VALUES ($1, $2)`, assignment, student)
	}
	return assignment
}

func (w *dueWorld) override(assignment, student string, closes *time.Time, extra int) {
	w.t.Helper()
	w.exec(`INSERT INTO app.assignment_student_overrides (assignment_id, student_id, closes_at, extra_attempts, reason)
		VALUES ($1, $2, $3, $4, 'Lý do')`, assignment, student, closes, extra)
}

func (w *dueWorld) attempt(assignment, student, status string, deadline time.Time) string {
	w.t.Helper()
	started := deadline.Add(-time.Hour)
	var submitted, graded *time.Time
	var void *string
	switch status {
	case "submitted", "timed_out":
		submitted = &deadline
	case "graded":
		submitted, graded = &deadline, &deadline
	case "voided":
		submitted = &deadline
		void = new("Hủy bài")
	}
	number := w.id(`SELECT (coalesce(max(attempt_no), 0) + 1)::text FROM app.attempts WHERE assignment_id = $1 AND student_id = $2`, assignment, student)
	return w.id(`INSERT INTO app.attempts
		(assignment_id, test_version_id, student_id, attempt_no, status, session_id, shuffle_seed, beacon_token_hash,
		 started_at, deadline_at, submitted_at, graded_at, void_reason)
		VALUES ($1, (SELECT test_version_id FROM app.assignments WHERE id = $1), $2, $3::smallint, $4::app.attempt_status,
		        gen_random_uuid(), 1, sha256('x'::bytea), $5, $6, $7, $8, $9) RETURNING id::text`,
		assignment, student, number, status, started, deadline, submitted, graded, void)
}

func (w *dueWorld) pending(attempt string) {
	w.t.Helper()
	section := w.id(`INSERT INTO app.test_version_sections (test_version_id, ordinal, title)
		SELECT test_version_id, 0, 'Phần 1' FROM app.attempts WHERE id = $1 RETURNING id::text`, attempt)
	question := w.id(`INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, points)
		VALUES ($1, 0, 'short_answer', 'Tả buổi sáng', 5) RETURNING id::text`, section)
	w.exec(`INSERT INTO app.attempt_answers (attempt_id, question_id, payload, requires_manual)
		VALUES ($1, $2, '{"type":"text","value":"Tôi dậy sớm."}', true)`, attempt, question)
}

func (w *dueWorld) due(user string) []domain.Notice {
	return w.dueAt(user, w.now)
}

func (w *dueWorld) dueAt(user string, now time.Time) []domain.Notice {
	w.t.Helper()
	notices, err := w.store.Due(context.Background(), user, now)
	if err != nil {
		w.t.Fatalf("Due: %v", err)
	}
	for _, n := range notices {
		if n.UserID != user {
			w.t.Errorf("Due(%s) returned a notice for %s", user, n.UserID)
		}
		if err := n.Validate(); err != nil {
			w.t.Errorf("Due returned a notice the store refuses: %v (%+v)", err, n)
		}
	}
	return notices
}

func ofKind(notices []domain.Notice, kind domain.Kind) []domain.Notice {
	var out []domain.Notice
	for _, n := range notices {
		if n.Kind == kind {
			out = append(out, n)
		}
	}
	return out
}

func keysOf(notices []domain.Notice) []string {
	keys := make([]string, len(notices))
	for i, n := range notices {
		keys[i] = n.DedupeKey
	}
	slices.Sort(keys)
	return keys
}

func at(offset time.Duration) time.Time { return instant.Add(offset) }

func moment(offset time.Duration) *time.Time {
	t := at(offset)
	return &t
}
