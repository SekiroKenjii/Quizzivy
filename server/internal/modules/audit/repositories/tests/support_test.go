//go:build integration

package repositories_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/audit/application"
	"quizzivy/internal/modules/audit/domain"
	"quizzivy/internal/modules/audit/repositories"
	"quizzivy/internal/platform/db"
)

var vietnam = domain.LocationOf(domain.DefaultZone)

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
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

func march(day, hour, minute int) time.Time {
	return time.Date(2001, time.March, day, hour, minute, 0, 0, vietnam)
}

func feb(day, hour, minute int) time.Time {
	return time.Date(2001, time.February, day, hour, minute, 0, 0, vietnam)
}

type world struct {
	t     *testing.T
	pool  *pgxpool.Pool
	repo  *repositories.Postgres
	app   *application.Application
	nonce string
}

func newWorld(t *testing.T) *world {
	t.Helper()
	pool := newPool(t)
	repo := repositories.NewPostgres(db.NewContext(pool))
	app := application.New(repo)
	app.SetClock(func() time.Time { return march(15, 12, 0) })
	return &world{t: t, pool: pool, repo: repo, app: app, nonce: nonce(t)}
}

func (w *world) exec(sql string, args ...any) {
	w.t.Helper()
	if _, err := w.pool.Exec(context.Background(), sql, args...); err != nil {
		w.t.Fatalf("%s: %v", sql, err)
	}
}

func (w *world) insertID(sql string, cleanup string, args ...any) string {
	w.t.Helper()
	var id string
	if err := w.pool.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		w.t.Fatalf("%s: %v", sql, err)
	}
	w.t.Cleanup(func() {
		if _, err := w.pool.Exec(context.Background(), cleanup, id); err != nil {
			w.t.Errorf("removing %s: %v", cleanup, err)
		}
	})
	return id
}

func (w *world) user(builtin, fullName, displayName string) string {
	w.t.Helper()
	var display any
	if displayName != "" {
		display = displayName
	}
	return w.insertID(`
		INSERT INTO app.users (email, full_name, display_name, role_id)
		VALUES ($1, $2, $3, (SELECT id FROM app.roles WHERE builtin_key = $4))
		RETURNING id::text`,
		`DELETE FROM app.users WHERE id = $1::uuid`,
		"audit-"+nonce(w.t)+"@example.com", fullName, display, builtin)
}

func (w *world) class(name, teacher string) string {
	w.t.Helper()
	return w.insertID(`INSERT INTO app.classes (name, teacher_id) VALUES ($1, $2::uuid) RETURNING id::text`,
		`DELETE FROM app.classes WHERE id = $1::uuid`, name, teacher)
}

type entry struct {
	actor     string
	action    string
	entity    string
	entityID  string
	at        time.Time
	ip        string
	userAgent string
	diff      string
}

func (w *world) write(e entry) int64 {
	w.t.Helper()
	var actor, entityID, ip, userAgent, diff any
	for target, value := range map[*any]string{&actor: e.actor, &entityID: e.entityID, &ip: e.ip, &userAgent: e.userAgent, &diff: e.diff} {
		if value != "" {
			*target = value
		}
	}
	if e.action == "" {
		e.action = "class.created"
	}
	if e.entity == "" {
		e.entity = "class"
	}
	var id int64
	err := w.pool.QueryRow(context.Background(), `
		INSERT INTO app.audit_log (actor_user_id, action, entity, entity_id, occurred_at, ip, user_agent, diff)
		VALUES ($1::uuid, $2, $3, $4::uuid, $5, $6::inet, $7, $8::jsonb)
		RETURNING id`, actor, e.action, e.entity, entityID, e.at, ip, userAgent, diff).Scan(&id)
	if err != nil {
		w.t.Fatalf("audit row %+v: %v", e, err)
	}
	w.t.Cleanup(func() {
		if _, err := w.pool.Exec(context.Background(), `DELETE FROM app.audit_log WHERE id = $1`, id); err != nil {
			w.t.Errorf("removing audit row %d: %v", id, err)
		}
	})
	return id
}

func (w *world) march() domain.Filter {
	from, to := march(1, 0, 0), time.Date(2001, time.April, 1, 0, 0, 0, 0, vietnam)
	return domain.Filter{Window: domain.Window{From: &from, To: &to}}
}
