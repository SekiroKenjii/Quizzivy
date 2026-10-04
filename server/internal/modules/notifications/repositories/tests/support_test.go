//go:build integration

package repositories_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/notifications/application"
	"quizzivy/internal/modules/notifications/application/command"
	"quizzivy/internal/modules/notifications/application/query"
	"quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/modules/notifications/repositories"
	"quizzivy/internal/platform/db"
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

func newUser(t *testing.T, pool *pgxpool.Pool, builtin string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO app.users (email, full_name, role_id)
		VALUES ($1, 'Người nhận thông báo', (SELECT id FROM app.roles WHERE builtin_key = $2))
		RETURNING id::text`, "thong-bao-"+nonce(t)+"@example.com", builtin).Scan(&id); err != nil {
		t.Fatalf("user: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM app.users WHERE id = $1::uuid`, id); err != nil {
			t.Errorf("removing the user: %v", err)
		}
	})
	return id
}

func over(pool *pgxpool.Pool) *application.Application {
	return application.New(repositories.NewPostgres(db.NewContext(pool)))
}

func notify(t *testing.T, app *application.Application, userID, key string, params domain.Params, merge domain.Merge) {
	t.Helper()
	if _, err := app.Commands.Notify.Handle(context.Background(), command.Notify{
		UserID: userID, Kind: params.Kind(), Params: params, DedupeKey: key, Merge: merge,
	}); err != nil {
		t.Fatalf("notify %s: %v", key, err)
	}
}

func fill(t *testing.T, app *application.Application, userID string, n int) {
	t.Helper()
	for i := range n {
		notify(t, app, userID, "joined:"+nonce(t), domain.Joined{StudentName: "Nguyễn Gia Bảo", ClassName: "Lớp " + string(rune('A'+i%26))}, domain.Replace)
	}
}

type stored struct {
	id        string
	kind      string
	params    map[string]any
	target    map[string]any
	createdAt time.Time
	updatedAt time.Time
	readAt    *time.Time
}

func rowsOf(t *testing.T, pool *pgxpool.Pool, userID string) []stored {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT id::text, kind, params, target, created_at, updated_at, read_at
		  FROM app.notifications WHERE user_id = $1::uuid ORDER BY id`, userID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []stored
	for rows.Next() {
		var (
			s              stored
			params, target []byte
		)
		if err := rows.Scan(&s.id, &s.kind, &params, &target, &s.createdAt, &s.updatedAt, &s.readAt); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(params, &s.params); err != nil {
			t.Fatalf("params %s: %v", params, err)
		}
		if target != nil {
			if err := json.Unmarshal(target, &s.target); err != nil {
				t.Fatalf("target %s: %v", target, err)
			}
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func theRow(t *testing.T, pool *pgxpool.Pool, userID string) stored {
	t.Helper()
	rows := rowsOf(t, pool, userID)
	if len(rows) != 1 {
		t.Fatalf("the user holds %d notifications, want exactly one", len(rows))
	}
	return rows[0]
}

func idsOf(rows []stored) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.id
	}
	return out
}

func unread(t *testing.T, app *application.Application, userID string) int {
	t.Helper()
	summary, err := app.Queries.Summary.Handle(context.Background(), query.Summary{UserID: userID})
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	return summary.UnreadNotifications
}

func page(t *testing.T, app *application.Application, userID, before string, limit int) domain.Page {
	t.Helper()
	got, err := app.Queries.List.Handle(context.Background(), query.List{UserID: userID, Before: before, Limit: limit})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	return got
}

func listed(p domain.Page) []string {
	out := make([]string, len(p.Items))
	for i, n := range p.Items {
		out[i] = n.ID
	}
	return out
}
