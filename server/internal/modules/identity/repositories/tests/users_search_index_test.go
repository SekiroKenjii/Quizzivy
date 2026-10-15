//go:build integration

package repositories_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/modules/identity/repositories"
	"quizzivy/internal/platform/db"
)

const usersSearchIndex = "users_search_trgm_idx"

func searchIndexPool(t *testing.T) *pgxpool.Pool {
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
	return pool
}

func TestTheUsersSearchExpressionIsTheOneTheTrigramIndexServes(t *testing.T) {
	ctx := context.Background()
	tx, err := searchIndexPool(t).Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	if _, err := tx.Exec(ctx, `SET LOCAL enable_seqscan = off`); err != nil {
		t.Fatal(err)
	}

	rows, err := tx.Query(ctx, `EXPLAIN SELECT u.id FROM app.users u WHERE `+repositories.UserSearchText+
		` LIKE '%' || app.immutable_unaccent(lower('nghe')) || '%' ESCAPE '\'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		plan.WriteString(line + "\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan.String(), "Bitmap Index Scan on "+usersSearchIndex) {
		t.Errorf("the planner does not choose %s for the Users search, so the index no longer matches the query:\n%s", usersSearchIndex, plan.String())
	}
}

func TestTheUsersSearchIndexIsValid(t *testing.T) {
	var valid bool
	if err := searchIndexPool(t).QueryRow(context.Background(), `
		SELECT indisvalid FROM pg_index WHERE indexrelid = to_regclass('app.users_search_trgm_idx')`).Scan(&valid); err != nil {
		t.Fatalf("read %s: %v", usersSearchIndex, err)
	}
	if !valid {
		t.Errorf("%s is INVALID: an interrupted build left it, which the planner ignores", usersSearchIndex)
	}
}

type sentStatement struct {
	sql  string
	args []any
}

type recordingConn struct {
	pgx.Tx
	sent []sentStatement
}

func (c *recordingConn) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	c.sent = append(c.sent, sentStatement{sql, args})
	return c.Tx.Query(ctx, sql, args...)
}

func (c *recordingConn) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	c.sent = append(c.sent, sentStatement{sql, args})
	return c.Tx.QueryRow(ctx, sql, args...)
}

func TestTheStatementsTheUsersDirectorySendsCanUseTheTrigramIndex(t *testing.T) {
	ctx := context.Background()
	tx, err := searchIndexPool(t).Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	for _, setting := range []string{"enable_seqscan", "enable_indexscan"} {
		if _, err := tx.Exec(ctx, `SET LOCAL `+setting+` = off`); err != nil {
			t.Fatal(err)
		}
	}
	conn := &recordingConn{Tx: tx}
	users := repositories.NewAdminUsers(db.NewContext(conn), nil)
	in := domain.AdminUserQuery{Query: "nghe", Page: 1, Limit: 20}
	if _, _, err := users.List(ctx, in); err != nil {
		t.Fatal(err)
	}
	if _, err := users.Facets(ctx, in); err != nil {
		t.Fatal(err)
	}

	searches := 0
	for _, sent := range conn.sent {
		if !strings.Contains(sent.sql, "immutable_unaccent") {
			continue
		}
		searches++
		rows, err := tx.Query(ctx, `EXPLAIN `+sent.sql, sent.args...)
		if err != nil {
			t.Fatal(err)
		}
		var plan strings.Builder
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				t.Fatal(err)
			}
			plan.WriteString(line + "\n")
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(plan.String(), "Bitmap Index Scan on "+usersSearchIndex) {
			t.Errorf("the planner does not choose %s for a statement the Users directory sends:\n%s\n%s", usersSearchIndex, sent.sql, plan.String())
		}
	}
	if searches != 3 {
		t.Errorf("the directory sent %d statements with the search, want 3 (the count, the page and the facets)", searches)
	}
}
