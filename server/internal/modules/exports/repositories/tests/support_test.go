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

	"quizzivy/internal/modules/exports/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/email"
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
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

type world struct {
	t    *testing.T
	pool *pgxpool.Pool
	repo *repositories.Postgres
}

func newWorld(t *testing.T) *world {
	t.Helper()
	pool := newPool(t)
	w := &world{t: t, pool: pool, repo: repositories.NewPostgres(db.NewContext(pool)).WithOutbox(email.NewOutbox(nil))}
	w.requireNoActiveExport()
	return w
}

func (w *world) requireNoActiveExport() {
	w.t.Helper()
	var active int
	if err := w.pool.QueryRow(context.Background(), `SELECT count(*) FROM app.data_exports WHERE status IN ('queued', 'running')`).Scan(&active); err != nil {
		w.t.Fatal(err)
	}
	if active != 0 {
		w.t.Fatalf("%d exports are queued or running in the test database; these tests need it idle", active)
	}
}

func (w *world) exec(sql string, args ...any) {
	w.t.Helper()
	if _, err := w.pool.Exec(context.Background(), sql, args...); err != nil {
		w.t.Fatalf("%s: %v", sql, err)
	}
}

func (w *world) user(builtin, fullName string) string {
	w.t.Helper()
	var id string
	err := w.pool.QueryRow(context.Background(), `
		INSERT INTO app.users (email, full_name, role_id)
		VALUES ($1, $2, (SELECT id FROM app.roles WHERE builtin_key = $3))
		RETURNING id::text`, "export-"+nonce(w.t)+"@example.com", fullName, builtin).Scan(&id)
	if err != nil {
		w.t.Fatalf("user: %v", err)
	}
	w.t.Cleanup(func() {
		if _, err := w.pool.Exec(context.Background(), `DELETE FROM app.users WHERE id = $1::uuid`, id); err != nil {
			w.t.Errorf("removing the user: %v", err)
		}
	})
	w.t.Cleanup(func() {
		if _, err := w.pool.Exec(context.Background(), `DELETE FROM app.audit_log WHERE actor_user_id = $1::uuid`, id); err != nil {
			w.t.Errorf("removing the audit rows: %v", err)
		}
	})
	return id
}

func (w *world) admin() string { return w.user("teacher", "Người yêu cầu "+nonce(w.t)) }

type seed struct {
	status         string
	attempts       int
	ranFor         time.Duration
	includeAnswers bool
	expiresIn      time.Duration
	downloads      int
	errorCode      string
	age            time.Duration
}

func (w *world) export(requester string, s seed) string {
	w.t.Helper()
	var id string
	err := w.pool.QueryRow(context.Background(), `
		INSERT INTO app.data_exports
		       (requested_by, include_answers, status, attempts, started_at, completed_at, expires_at,
		        downloaded_at, downloads, storage_key, bytes, error_code, created_at)
		VALUES ($1::uuid, $2, $3, $4,
		        CASE WHEN $5::bigint > 0 THEN now() - $5 * interval '1 second' END,
		        CASE WHEN $3 IN ('ready', 'failed') THEN now() END,
		        CASE WHEN $3 = 'ready' THEN now() + $6 * interval '1 second' END,
		        CASE WHEN $7::int > 0 THEN now() END, $7::int,
		        CASE WHEN $3 = 'ready' THEN 'exports/seed-' || $8 || '.zip' END,
		        CASE WHEN $3 = 'ready' THEN 1234 END,
		        NULLIF($9, ''), now() - $10 * interval '1 second')
		RETURNING id::text`,
		requester, s.includeAnswers, s.status, s.attempts, int64(s.ranFor/time.Second), int64(s.expiresIn/time.Second),
		s.downloads, nonce(w.t), s.errorCode, int64(s.age/time.Second)).Scan(&id)
	if err != nil {
		w.t.Fatalf("seed an export: %v", err)
	}
	w.track(id)
	return id
}

type row struct {
	status       string
	attempts     int
	startedAt    *time.Time
	completedAt  *time.Time
	expiresAt    *time.Time
	downloadedAt *time.Time
	downloads    int
	storageKey   *string
	bytes        *int64
	errorCode    *string
}

func (w *world) row(id string) row {
	w.t.Helper()
	var r row
	err := w.pool.QueryRow(context.Background(), `
		SELECT status, attempts, started_at, completed_at, expires_at, downloaded_at, downloads, storage_key, bytes, error_code
		  FROM app.data_exports WHERE id = $1::uuid`, id).
		Scan(&r.status, &r.attempts, &r.startedAt, &r.completedAt, &r.expiresAt, &r.downloadedAt, &r.downloads, &r.storageKey, &r.bytes, &r.errorCode)
	if err != nil {
		w.t.Fatalf("read the export: %v", err)
	}
	return r
}

func (w *world) auditCount(action, id string) int {
	w.t.Helper()
	var n int
	if err := w.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM app.audit_log WHERE action = $1 AND entity = 'data_export' AND entity_id = $2::uuid`, action, id).Scan(&n); err != nil {
		w.t.Fatal(err)
	}
	return n
}

func (w *world) outboxFor(id string) []string {
	w.t.Helper()
	rows, err := w.pool.Query(context.Background(), `SELECT params::text FROM app.email_outbox WHERE dedupe_key = $1`, "data_export_ready:"+id)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			w.t.Fatal(err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		w.t.Fatal(err)
	}
	return out
}

func (w *world) track(id string) {
	w.t.Helper()
	w.t.Cleanup(func() {
		if _, err := w.pool.Exec(context.Background(), `DELETE FROM app.email_outbox WHERE dedupe_key = $1`, "data_export_ready:"+id); err != nil {
			w.t.Errorf("removing the outbox rows: %v", err)
		}
	})
}
