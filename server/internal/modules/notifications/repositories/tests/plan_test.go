//go:build integration

package repositories_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/modules/notifications/repositories"
	"quizzivy/internal/platform/db"
)

type statement struct {
	sql  string
	args []any
}

type recording struct {
	pool *pgxpool.Pool
	mu   sync.Mutex
	seen []statement
}

func (r *recording) keep(sql string, args []any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seen = append(r.seen, statement{sql, args})
}

func (r *recording) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	r.keep(sql, args)
	return r.pool.Exec(ctx, sql, args...)
}

func (r *recording) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	r.keep(sql, args)
	return r.pool.Query(ctx, sql, args...)
}

func (r *recording) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	r.keep(sql, args)
	return r.pool.QueryRow(ctx, sql, args...)
}

func (r *recording) Begin(ctx context.Context) (pgx.Tx, error) { return r.pool.Begin(ctx) }

func explained(t *testing.T, tx pgx.Tx, sql string, args ...any) string {
	t.Helper()
	rows, err := tx.Query(context.Background(), "EXPLAIN (COSTS OFF) "+sql, args...)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	lines, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	return strings.Join(lines, "\n")
}

func TestTheListAndTheCountAreGivenByTheirIndexes(t *testing.T) {
	pool := newPool(t)
	userID := newUser(t, pool, "teacher")
	conn := &recording{pool: pool}
	store := repositories.NewPostgres(db.NewContext(conn))
	ctx := context.Background()
	if _, err := store.List(ctx, domain.ListQuery{UserID: userID, Limit: 20}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.List(ctx, domain.ListQuery{UserID: userID, Before: uuid.NewString(), Limit: 20}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Unread(ctx, userID, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := store.List(ctx, domain.ListQuery{UserID: userID, Limit: 20, Audience: domain.AudienceTeacher}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.List(ctx, domain.ListQuery{UserID: userID, Before: uuid.NewString(), Limit: 20, Audience: domain.AudienceStudent}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Unread(ctx, userID, domain.AudienceTeacher); err != nil {
		t.Fatal(err)
	}
	wantArgs := []int{2, 3, 1, 3, 4, 2}
	if len(conn.seen) != len(wantArgs) {
		t.Fatalf("six reads were expected, ran %+v", conn.seen)
	}
	for i, want := range wantArgs {
		if len(conn.seen[i].args) != want {
			t.Fatalf("read %d took %d arguments, want %d: %+v", i, len(conn.seen[i].args), want, conn.seen[i])
		}
	}
	first, later, count := conn.seen[0].sql, conn.seen[1].sql, conn.seen[2].sql
	firstOf, laterOf, countOf := conn.seen[3].sql, conn.seen[4].sql, conn.seen[5].sql

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	batch := nonce(t)
	for _, statement := range []string{
		`INSERT INTO app.users (email, full_name, role_id)
		 SELECT 'ke-hoach-' || $1 || '-' || g || '@example.com', 'Người đọc ' || g, (SELECT id FROM app.roles WHERE builtin_key = 'teacher')
		   FROM generate_series(1, 200) g`,
		`INSERT INTO app.notifications (user_id, kind, params, dedupe_key, read_at)
		 SELECT u.id, CASE WHEN g % 3 = 0 THEN 'result.ready' ELSE 'class.joined' END,
		        '{"studentName":"Bảo","className":"Lớp A"}'::jsonb, 'k' || g, CASE WHEN g % 10 = 0 THEN NULL ELSE now() END
		   FROM app.users u, generate_series(1, 250) g
		  WHERE u.email LIKE 'ke-hoach-' || $1 || '-%'`,
	} {
		if _, err := tx.Exec(ctx, statement, batch); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tx.Exec(ctx, "ANALYZE app.notifications"); err != nil {
		t.Fatal(err)
	}
	var reader, middle string
	if err := tx.QueryRow(ctx, `
		SELECT n.user_id::text, n.id::text FROM app.notifications n JOIN app.users u ON u.id = n.user_id
		 WHERE u.email = 'ke-hoach-' || $1 || '-7@example.com' ORDER BY n.id OFFSET 125 LIMIT 1`, batch).Scan(&reader, &middle); err != nil {
		t.Fatal(err)
	}

	for name, plan := range map[string]string{
		"the first page": explained(t, tx, first, reader, 21),
		"a later page":   explained(t, tx, later, reader, middle, 21),
	} {
		if !strings.Contains(plan, "Index Scan using notifications_user_recent_idx") || strings.Contains(plan, "Sort") || strings.Contains(plan, "Filter") {
			t.Errorf("%s is not read from notifications_user_recent_idx alone, in its order:\n%s", name, plan)
		}
	}
	if plan := explained(t, tx, later, reader, middle, 21); !strings.Contains(plan, "(id < ") {
		t.Errorf("the cursor is not an index condition:\n%s", plan)
	}
	if plan := explained(t, tx, count, reader); !strings.Contains(plan, "notifications_unread_idx") {
		t.Errorf("the unread count does not use notifications_unread_idx:\n%s", plan)
	}

	teacher, student := []string{"attempt.submitted", "class.joined"}, []string{"result.ready"}
	for name, plan := range map[string]string{
		"the first page of an audience": explained(t, tx, firstOf, reader, teacher, 21),
		"a later page of an audience":   explained(t, tx, laterOf, reader, middle, student, 21),
	} {
		if !strings.Contains(plan, "Index Scan using notifications_user_recent_idx") || strings.Contains(plan, "Sort") || !strings.Contains(plan, "kind = ANY") {
			t.Errorf("%s is not read from notifications_user_recent_idx in its order with the kind as a filter:\n%s", name, plan)
		}
	}
	if plan := explained(t, tx, laterOf, reader, middle, student, 21); !strings.Contains(plan, "(id < ") {
		t.Errorf("the cursor of an audience's page is not an index condition:\n%s", plan)
	}
	if plan := explained(t, tx, countOf, reader, teacher); !strings.Contains(plan, "notifications_unread_idx") {
		t.Errorf("the unread count of an audience does not use notifications_unread_idx:\n%s", plan)
	}
}
