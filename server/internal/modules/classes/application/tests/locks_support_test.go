//go:build integration

package application_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/classes/application"
	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/application/query"
	"quizzivy/internal/modules/classes/domain"
)

func finished[T any](t *testing.T, what string, done <-chan T) T {
	t.Helper()
	select {
	case got := <-done:
		return got
	case <-time.After(30 * time.Second):
		t.Fatalf("%s did not finish", what)
		panic("unreachable")
	}
}

type codeRow struct {
	id         string
	scheme     int16
	keyID      *int16
	hash       []byte
	ciphertext []byte
	hint       string
	expiresAt  time.Time
	maxUses    *int
	usesCount  int
	revokedAt  *time.Time
	createdBy  string
	createdAt  time.Time
}

func codeRows(t *testing.T, pool *pgxpool.Pool, classID string) []codeRow {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT id::text, lookup_scheme, key_id, code_hash, code_ciphertext, code_hint,
		       expires_at, max_uses, uses_count, revoked_at, created_by::text, created_at
		  FROM app.class_join_codes
		 WHERE class_id = $1
		 ORDER BY (revoked_at IS NULL), id`, classID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []codeRow
	for rows.Next() {
		var c codeRow
		if err := rows.Scan(&c.id, &c.scheme, &c.keyID, &c.hash, &c.ciphertext, &c.hint,
			&c.expiresAt, &c.maxUses, &c.usesCount, &c.revokedAt, &c.createdBy, &c.createdAt); err != nil {
			t.Fatal(err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func ownConnection(t *testing.T, settings ...string) (*pgxpool.Pool, int) {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("pool config: %v", err)
	}
	cfg.MaxConns = 1
	for i := 0; i+1 < len(settings); i += 2 {
		cfg.ConnConfig.RuntimeParams[settings[i]] = settings[i+1]
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	var pid int
	if err := pool.QueryRow(context.Background(), `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatalf("read the connection's backend: %v", err)
	}
	return pool, pid
}

type heldTx struct {
	t    *testing.T
	tx   pgx.Tx
	pid  int
	done bool
}

func newHeldTx(t *testing.T, pool *pgxpool.Pool, statement string, args ...any) *heldTx {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin the holder: %v", err)
	}
	h := &heldTx{t: t, tx: tx}
	t.Cleanup(h.rollback)
	if _, err := tx.Exec(ctx, statement, args...); err != nil {
		t.Fatalf("take the lock to hold: %v", err)
	}
	if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&h.pid); err != nil {
		t.Fatalf("read the holder's backend: %v", err)
	}
	return h
}

func (h *heldTx) rollback() {
	if h.done {
		return
	}
	h.done = true
	if err := h.tx.Rollback(context.Background()); err != nil {
		h.t.Errorf("release the holder: %v", err)
	}
}

func (h *heldTx) commit() error {
	h.done = true
	return h.tx.Commit(context.Background())
}

func waitUntilBlocked(t *testing.T, pool *pgxpool.Pool, waiter, blocker int, what string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		var blocked bool
		if err := pool.QueryRow(context.Background(),
			`SELECT $2::int = ANY(pg_blocking_pids($1::int))`, waiter, blocker).Scan(&blocked); err != nil {
			t.Fatalf("look for %s: %v", what, err)
		}
		if blocked {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s never waited", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

type refusal struct {
	preview  domain.PreviewResult
	existing domain.EnrolResult
	google   domain.EnrolResult
}

func refusalOf(t *testing.T, pool *pgxpool.Pool, svc *application.Application, code string) refusal {
	t.Helper()
	ctx := context.Background()
	_, _, student := makeClassRow(t, pool)
	existing, err := svc.Commands.EnrolExisting.Handle(ctx, command.EnrolExisting{UserID: student, Code: code})
	if err != nil {
		t.Fatalf("joining as a signed-in student: %v", err)
	}
	m := newMember(t)
	dropUser(t, pool, m.Email)
	google, err := svc.Commands.EnrolNewMember.Handle(ctx, command.EnrolNewMember{Member: m, Code: code})
	if err != nil {
		t.Fatalf("the Google join: %v", err)
	}
	var accounts int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM app.users WHERE email = $1`, m.Email).Scan(&accounts); err != nil {
		t.Fatal(err)
	}
	if accounts != 0 {
		t.Errorf("a refused Google join left %d account(s) behind", accounts)
	}
	return refusal{preview: previewOf(t, svc, code), existing: existing, google: google}
}

func refusedAs(outcome domain.PreviewOutcome) refusal {
	return refusal{
		preview:  domain.PreviewResult{Outcome: outcome},
		existing: domain.EnrolResult{Outcome: outcome},
		google:   domain.EnrolResult{Outcome: outcome},
	}
}

func holding(t *testing.T, pool *pgxpool.Pool, statement string, args ...any) (release func(), pid int) {
	t.Helper()
	h := newHeldTx(t, pool, statement, args...)
	return h.rollback, h.pid
}

func currentCode(t *testing.T, svc *application.Application, classID string) domain.ActiveJoinCode {
	t.Helper()
	got, err := svc.Queries.ActiveCode.Handle(context.Background(), query.ActiveCode{Scope: everyone, ClassID: classID})
	if err != nil {
		t.Fatalf("read the active code back: %v", err)
	}
	if got.Legacy || got.Code == "" {
		t.Fatalf("the active code of %s reads back as %+v, want a sealed code its teacher can read", classID, got)
	}
	return got
}
