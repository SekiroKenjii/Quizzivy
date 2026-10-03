//go:build integration

package repositories_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/modules/classes/repositories"
	"quizzivy/internal/platform/db"
)

func restrictingKeys(t *testing.T, tables string) map[string]bool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), db.TestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	rows, err := pool.Query(context.Background(), `SELECT c.conname, c.condeferrable FROM pg_catalog.pg_constraint c
		 WHERE c.contype = 'f' AND c.confrelid::regclass::text = ANY(string_to_array($1, ',')) AND c.confdeltype IN ('a', 'r')`, tables)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var name string
		var deferrable bool
		if err := rows.Scan(&name, &deferrable); err != nil {
			t.Fatal(err)
		}
		out[name] = deferrable
	}
	if err := rows.Err(); err != nil || len(out) == 0 {
		t.Fatalf("restricting keys on %s: %v (%v)", tables, out, err)
	}
	return out
}

func TestEveryKeyThatRefusesAClassesDeletionIsNamed(t *testing.T) {
	for name, deferrable := range restrictingKeys(t, "app.classes,app.class_join_codes") {
		if repositories.ClassReferencedBy(name) == domain.ReferencedByOther {
			t.Errorf("%s refuses a class's deletion but the map does not name it", name)
		}
		if deferrable {
			t.Errorf("%s is deferrable, so its refusal would arrive at commit, unmapped", name)
		}
	}
}
