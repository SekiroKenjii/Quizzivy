//go:build integration

package application_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/ai/application"
	"quizzivy/internal/modules/ai/application/query"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/modules/ai/repositories"
	notificationsapp "quizzivy/internal/modules/notifications/application"
	notificationsrepo "quizzivy/internal/modules/notifications/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/actor"
)

var monthCounter atomic.Int64

func init() {
	var seed [2]byte
	if _, err := rand.Read(seed[:]); err != nil {
		panic(err)
	}
	monthCounter.Store(int64(seed[0])<<4 + int64(seed[1])&0x0f)
}

type world struct {
	t      *testing.T
	pool   db.Conn
	app    *application.Application
	clock  atomic.Pointer[time.Time]
	mu     sync.Mutex
	months []time.Time
	users  []string
	keys   []string
	saved  *domain.Rules
}

func nonce(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

func connect(t *testing.T) *pgxpool.Pool {
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

func newWorld(t *testing.T) *world {
	t.Helper()
	w := &world{t: t, pool: connect(t)}
	t.Cleanup(w.remove)
	w.build()
	return w
}

func newTxWorld(t *testing.T) *world {
	t.Helper()
	ctx := context.Background()
	tx, err := connect(t).Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("rollback: %v", err)
		}
	})
	w := &world{t: t, pool: tx}
	w.build()
	return w
}

func (w *world) build() {
	dbx := db.NewContext(w.pool)
	notifications := notificationsapp.New(notificationsrepo.NewPostgres(dbx))
	w.app = application.New(repositories.NewPostgres(dbx), application.Options{
		Prices: testPrices, Zone: saigon, Notifier: notifications.Commands.Notify, Logger: slog.New(slog.DiscardHandler),
		People: everyoneAlike{},
	})
	w.app.SetClock(func() time.Time { return *w.clock.Load() })
	w.rememberRules()
}

func (w *world) exec(sql string, args ...any) {
	w.t.Helper()
	if _, err := w.pool.Exec(context.Background(), sql, args...); err != nil {
		w.t.Fatalf("%s: %v", sql, err)
	}
}

func (w *world) remove() {
	ctx := context.Background()
	w.mu.Lock()
	months, users, keys := w.months, w.users, w.keys
	w.mu.Unlock()
	for _, step := range []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM app.audit_log WHERE action LIKE 'ai.%' AND (actor_user_id = ANY($1::uuid[]) OR diff->>'month' = ANY($2::text[]))`, []any{users, monthNames(months)}},
		{`DELETE FROM app.ai_reservations WHERE month = ANY($1::date[])`, []any{months}},
		{`DELETE FROM app.ai_usage_ledger WHERE month = ANY($1::date[])`, []any{months}},
		{`DELETE FROM app.ai_credit_requests WHERE month = ANY($1::date[])`, []any{months}},
		{`DELETE FROM app.ai_teacher_credits WHERE month = ANY($1::date[])`, []any{months}},
		{`DELETE FROM app.ai_credit_periods WHERE month = ANY($1::date[])`, []any{months}},
		{`DELETE FROM app.notifications WHERE dedupe_key = ANY($1::text[])`, []any{keys}},
		{`DELETE FROM app.users WHERE id = ANY($1::uuid[])`, []any{users}},
	} {
		if _, err := w.pool.Exec(ctx, step.sql, step.args...); err != nil {
			w.t.Errorf("cleanup %q: %v", step.sql, err)
		}
	}
	if w.saved != nil {
		if _, err := w.pool.Exec(ctx, `
			UPDATE app.ai_settings SET default_allowance_credits = $1, warn_percent = $2, on_teacher_limit = $3`,
			w.saved.DefaultAllowance, w.saved.WarnPercent, string(w.saved.OnTeacherLimit)); err != nil {
			w.t.Errorf("restoring the rules: %v", err)
		}
	}
}

func monthNames(months []time.Time) []string {
	out := make([]string, len(months))
	for i, m := range months {
		out[i] = m.Format(time.DateOnly)
	}
	return out
}

func (w *world) rememberRules() {
	rules, err := repositories.NewPostgres(db.NewContext(w.pool)).Rules(context.Background())
	if err != nil {
		w.t.Fatal(err)
	}
	w.saved = &rules
}

func (w *world) rules() domain.Rules {
	w.t.Helper()
	rules, err := repositories.NewPostgres(db.NewContext(w.pool)).Rules(context.Background())
	if err != nil {
		w.t.Fatal(err)
	}
	return rules
}

func (w *world) track(months ...time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.months = append(w.months, months...)
}

func (w *world) watch(keys ...string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.keys = append(w.keys, keys...)
}

func (w *world) newMonth() time.Time {
	n := monthCounter.Add(1)
	month := time.Date(2300, time.Month(1+n%12), 1, 0, 0, 0, 0, time.UTC).AddDate(int(n/12), 0, 0)
	w.track(month)
	w.at(month.Add(14 * 24 * time.Hour))
	return month
}

func (w *world) at(t time.Time) {
	w.clock.Store(&t)
}

func (w *world) user(builtin string) string {
	w.t.Helper()
	var id string
	if err := w.pool.QueryRow(context.Background(), `
		INSERT INTO app.users (email, full_name, role_id)
		VALUES ($1, 'Giáo viên thử credit', (SELECT id FROM app.roles WHERE builtin_key = $2))
		RETURNING id::text`, "credits-"+nonce(w.t)+"@example.com", builtin).Scan(&id); err != nil {
		w.t.Fatalf("user: %v", err)
	}
	w.mu.Lock()
	w.users = append(w.users, id)
	w.mu.Unlock()
	return id
}

func (w *world) teacher() string { return w.user("teacher") }

func (w *world) period(month time.Time, pool int64) {
	w.t.Helper()
	w.track(month)
	w.exec(`INSERT INTO app.ai_credit_periods (month, pool_credits) VALUES ($1::date, $2)`, month, pool)
}

func (w *world) allowance(month time.Time, userID string, allowance int64) {
	w.t.Helper()
	w.exec(`
		INSERT INTO app.ai_credit_periods (month, pool_credits)
		SELECT $1::date, pool_credits FROM app.ai_settings ON CONFLICT (month) DO NOTHING`, month)
	w.exec(`INSERT INTO app.ai_teacher_credits (month, user_id, allowance_credits) VALUES ($1::date, $2::uuid, $3)`, month, userID, allowance)
}

type teacherRow struct {
	allowance, granted, used, reserved int64
	custom, paused                     bool
	warned                             bool
}

func (w *world) teacherRow(month time.Time, userID string) teacherRow {
	w.t.Helper()
	var r teacherRow
	if err := w.pool.QueryRow(context.Background(), `
		SELECT allowance_credits, granted_credits, used_credits, reserved_credits, custom, paused, warned_at IS NOT NULL
		  FROM app.ai_teacher_credits WHERE month = $1::date AND user_id = $2::uuid`, month, userID).
		Scan(&r.allowance, &r.granted, &r.used, &r.reserved, &r.custom, &r.paused, &r.warned); err != nil {
		w.t.Fatalf("teacher row: %v", err)
	}
	return r
}

func (w *world) hasTeacherRow(month time.Time, userID string) bool {
	w.t.Helper()
	var n int
	if err := w.pool.QueryRow(context.Background(), `SELECT count(*) FROM app.ai_teacher_credits WHERE month = $1::date AND user_id = $2::uuid`, month, userID).Scan(&n); err != nil {
		w.t.Fatal(err)
	}
	return n > 0
}

type periodRow struct{ pool, added, used, reserved int64 }

func (w *world) periodRow(month time.Time) periodRow {
	w.t.Helper()
	var r periodRow
	if err := w.pool.QueryRow(context.Background(), `
		SELECT pool_credits, added_credits, used_credits, reserved_credits FROM app.ai_credit_periods WHERE month = $1::date`, month).
		Scan(&r.pool, &r.added, &r.used, &r.reserved); err != nil {
		w.t.Fatalf("period row: %v", err)
	}
	return r
}

func (w *world) count(sql string, args ...any) int {
	w.t.Helper()
	var n int
	if err := w.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		w.t.Fatalf("%s: %v", sql, err)
	}
	return n
}

func (w *world) reservations(month time.Time) int {
	return w.count(`SELECT count(*) FROM app.ai_reservations WHERE month = $1::date`, month)
}

func asActor(id string) actor.Actor { return actor.Actor{ID: id} }

func isScope(err error, scope domain.Scope) bool {
	var exhausted *domain.Exhausted
	return errors.As(err, &exhausted) && exhausted.Scope == scope && errors.Is(err, domain.ErrExhausted)
}

func usageOf(userID string) query.Usage { return query.Usage{UserID: userID} }
