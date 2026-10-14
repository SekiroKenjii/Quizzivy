//go:build integration

package email_test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/email"
	"quizzivy/internal/platform/secrets"
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

func sealerFor(t *testing.T) *secrets.Sealer {
	t.Helper()
	key, err := secrets.ParseKey(base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	if err != nil {
		t.Fatal(err)
	}
	sealer, err := secrets.New(key, secrets.PurposeEmailOutbox)
	if err != nil {
		t.Fatal(err)
	}
	return sealer
}

func newUser(t *testing.T, pool *pgxpool.Pool, locale string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO app.users (email, full_name, role_id, locale)
		VALUES ($1, 'Người nhận thư', (SELECT id FROM app.roles WHERE builtin_key = 'teacher'), $2)
		RETURNING id::text`, "thu-"+nonce(t)+"@example.com", locale).Scan(&id); err != nil {
		t.Fatalf("user: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM app.users WHERE id = $1::uuid`, id); err != nil {
			t.Errorf("removing the user: %v", err)
		}
	})
	return id
}

type fakeSender struct {
	mu       sync.Mutex
	sent     []email.Message
	failWith map[string]error
	delay    time.Duration
}

func (f *fakeSender) Send(_ context.Context, m email.Message) (string, error) {
	time.Sleep(f.delay)
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.failWith[m.To]; err != nil {
		return "", err
	}
	f.sent = append(f.sent, m)
	return "fake:" + m.IdempotencyKey, nil
}

func (f *fakeSender) to(address string) []email.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []email.Message
	for _, m := range f.sent {
		if m.To == address {
			out = append(out, m)
		}
	}
	return out
}

type row struct {
	id                                string
	attempts                          int
	sentAt, failedAt                  *time.Time
	deliverAfter                      time.Time
	hasSecret                         bool
	lastError, provider, locale, tmpl string
}

func read(t *testing.T, pool *pgxpool.Pool, id string) row {
	t.Helper()
	r := row{id: id}
	var lastError, provider *string
	if err := pool.QueryRow(context.Background(), `
		SELECT attempts, sent_at, failed_at, deliver_after, secret_ciphertext IS NOT NULL, last_error_code, provider_id, locale, template
		  FROM app.email_outbox WHERE id = $1::uuid`, id).Scan(
		&r.attempts, &r.sentAt, &r.failedAt, &r.deliverAfter, &r.hasSecret, &lastError, &provider, &r.locale, &r.tmpl); err != nil {
		t.Fatalf("reading %s: %v", id, err)
	}
	if lastError != nil {
		r.lastError = *lastError
	}
	if provider != nil {
		r.provider = *provider
	}
	return r
}

type world struct {
	pool   *pgxpool.Pool
	outbox *email.Outbox
	sender *fakeSender
	disp   *email.Dispatcher
	ids    []string
}

func newWorld(t *testing.T, now time.Time) *world {
	t.Helper()
	pool := newPool(t)
	sealer := sealerFor(t)
	renderer, err := email.NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	w := &world{pool: pool, outbox: email.NewOutbox(sealer), sender: &fakeSender{failWith: map[string]error{}}}
	w.disp = &email.Dispatcher{
		DB: db.NewContext(pool), Sealer: sealer, Sender: w.sender, Renderer: renderer,
		Org:    func(context.Context) (string, error) { return "Trung tâm Ánh Dương", nil },
		AppURL: "https://app.quizzivy.com", DailyLimit: 100, Location: time.FixedZone("ICT", 7*3600),
		Now: func() time.Time { return now }, Logger: slog.New(slog.DiscardHandler),
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM app.email_outbox WHERE id = ANY($1::uuid[])`, w.ids); err != nil {
			t.Errorf("removing the outbox rows: %v", err)
		}
	})
	return w
}

func (w *world) enqueue(t *testing.T, m email.Mail) string {
	t.Helper()
	tx, err := w.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := w.outbox.Enqueue(context.Background(), tx, m)
	if err != nil {
		_ = tx.Rollback(context.Background())
		t.Fatalf("enqueue: %v", err)
	}
	w.ids = append(w.ids, id)
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	return id
}

func invite(address string) email.Mail {
	return email.Mail{To: address, Params: email.AccountInviteParams{FullName: "An"}, Secret: "tok-" + address}
}
