//go:build integration

package repositories_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/identity/repositories"
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
