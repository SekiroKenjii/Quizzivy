//go:build integration

package application_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/core/adapters"
	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/application/ports"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/modules/attempts/repositories"
	notificationsapp "quizzivy/internal/modules/notifications/application"
	notificationscommand "quizzivy/internal/modules/notifications/application/command"
	notificationsdomain "quizzivy/internal/modules/notifications/domain"
	notificationsrepo "quizzivy/internal/modules/notifications/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/cqrs"
)

type told struct {
	mu   sync.Mutex
	sent []notificationscommand.Notify
	err  error

	onFirst func()
}

func (c *told) Handle(_ context.Context, n notificationscommand.Notify) (cqrs.Nothing, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.onFirst != nil {
		c.onFirst()
		c.onFirst = nil
	}
	if c.err != nil {
		return cqrs.Nothing{}, c.err
	}
	c.sent = append(c.sent, n)
	return cqrs.Nothing{}, nil
}

func (c *told) of(kind notificationsdomain.Kind) []notificationscommand.Notify {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []notificationscommand.Notify
	for _, n := range c.sent {
		if n.Kind == kind {
			out = append(out, n)
		}
	}
	return out
}

func (c *told) users(kind notificationsdomain.Kind) map[string]bool {
	out := map[string]bool{}
	for _, n := range c.of(kind) {
		out[n.UserID] = true
	}
	return out
}

func (c *told) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.sent)
}

func announcing(pool *pgxpool.Pool, notifier ports.Notifier, log *bytes.Buffer) *application.Application {
	dbx := db.NewContext(pool)
	logger := slog.New(slog.DiscardHandler)
	if log != nil {
		logger = slog.New(slog.NewTextHandler(log, nil))
	}
	return application.New(repositories.NewTimelines(dbx), repositories.NewReviews(dbx), repositories.NewPostgres(dbx, adapters.AttemptStartGuard{})).
		WithNotifier(notifier, logger)
}

func realNotifications(pool *pgxpool.Pool) *notificationsapp.Application {
	return notificationsapp.New(notificationsrepo.NewPostgres(db.NewContext(pool)))
}

type secondClass struct {
	teacher, class, student string
}

func addSecondClass(t *testing.T, pool *pgxpool.Pool, w world) *secondClass {
	t.Helper()
	ctx := context.Background()
	c := &secondClass{}
	t.Cleanup(func() {
		for _, q := range []struct{ sql, id string }{
			{`DELETE FROM app.attempt_events WHERE attempt_id IN (SELECT id FROM app.attempts WHERE student_id = $1::uuid)`, c.student},
			{`DELETE FROM app.attempt_answers WHERE attempt_id IN (SELECT id FROM app.attempts WHERE student_id = $1::uuid)`, c.student},
			{`DELETE FROM app.attempts WHERE student_id = $1::uuid`, c.student},
			{`DELETE FROM app.assignment_classes WHERE class_id = $1::uuid`, c.class},
			{`DELETE FROM app.class_members WHERE class_id = $1::uuid`, c.class},
			{`DELETE FROM app.classes WHERE id = $1::uuid`, c.class},
			{`DELETE FROM app.users WHERE id = $1::uuid`, c.student},
			{`DELETE FROM app.users WHERE id = $1::uuid`, c.teacher},
		} {
			if q.id == "" {
				continue
			}
			if _, err := pool.Exec(context.Background(), q.sql, q.id); err != nil {
				t.Errorf("cleanup %q: %v", q.sql, err)
			}
		}
	})
	id := uuid.NewString()
	if err := pool.QueryRow(ctx, `INSERT INTO app.users (email, full_name, role_id)
		VALUES ($1, 'Giáo viên lớp hai', (SELECT id FROM app.roles WHERE builtin_key = 'teacher')) RETURNING id::text`,
		"att-t2-"+id+"@example.com").Scan(&c.teacher); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO app.users (email, full_name, role_id)
		VALUES ($1, 'Học viên lớp hai', (SELECT id FROM app.roles WHERE builtin_key = 'student')) RETURNING id::text`,
		"att-s2-"+id+"@example.com").Scan(&c.student); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO app.classes (name, teacher_id) VALUES ('Lớp hai', $1::uuid) RETURNING id::text`, c.teacher).Scan(&c.class); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO app.class_members (class_id, user_id, joined_via, added_by) VALUES ($1::uuid, $2::uuid, 'admin', $2::uuid)`, c.class, c.student); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO app.assignment_classes (assignment_id, class_id) VALUES ($1::uuid, $2::uuid)`, w.assignment, c.class); err != nil {
		t.Fatal(err)
	}
	return c
}

func titleOf(t *testing.T, pool *pgxpool.Pool, w world) string {
	t.Helper()
	var title string
	if err := pool.QueryRow(context.Background(), `SELECT title FROM app.tests WHERE id = $1::uuid`, w.testID).Scan(&title); err != nil {
		t.Fatal(err)
	}
	return title
}

func startFor(t *testing.T, svc *application.Application, w world, student string) domain.Session {
	t.Helper()
	session, err := svc.Commands.StartOrResume.Handle(context.Background(), command.StartOrResume{AssignmentID: w.assignment, StudentID: student})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	return session
}

func hand(t *testing.T, svc *application.Application, session domain.Session, student string) {
	t.Helper()
	if _, err := svc.Commands.Submit.Handle(context.Background(), command.Submit{AttemptID: session.Attempt.ID, StudentID: student, Reason: domain.Manual}); err != nil {
		t.Fatalf("submit: %v", err)
	}
}

func correctOption(t *testing.T, pool *pgxpool.Pool, question string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`SELECT id::text FROM app.test_version_options WHERE test_version_question_id = $1::uuid AND is_correct`, question).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

var errNotifierDown = errors.New("the notifications store is away")
