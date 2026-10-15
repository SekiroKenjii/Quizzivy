//go:build integration

package router_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestTheLimiterKeyFoldsNothingApartThatTheLookupFoldsTogether(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	rows, err := pool.Query(ctx, `SELECT c, lower(chr(c)) FROM generate_series(32, 1114111) AS c WHERE c NOT BETWEEN 55296 AND 57343`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	postgresFold := map[string][]rune{}
	goFold := map[rune]string{}
	vietnamese := map[rune]string{}
	for rows.Next() {
		var code int32
		var folded string
		if err := rows.Scan(&code, &folded); err != nil {
			t.Fatal(err)
		}
		r := rune(code)
		postgresFold[folded] = append(postgresFold[folded], r)
		goFold[r] = strings.ToLower(string(r))
		if strings.ContainsRune("ĂÂĐÊÔƠƯẠẢẤẦẨẪẬẮẰẲẴẶẸẺẼẾỀỂỄỆỈỊỌỎỐỒỔỖỘỚỜỞỠỢỤỦỨỪỬỮỰỲỴỶỸ", r) {
			vietnamese[r] = folded
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	for folded, group := range postgresFold {
		for _, r := range group[1:] {
			if goFold[r] != goFold[group[0]] {
				t.Errorf("lower() folds %U and %U together to %q, but strings.ToLower keeps them apart as %q and %q: a spelling the lookup treats as one account would get a second budget",
					group[0], r, folded, goFold[group[0]], goFold[r])
			}
		}
	}
	if len(vietnamese) == 0 {
		t.Fatal("no Vietnamese capital was read, so the check proves nothing")
	}
	for r, folded := range vietnamese {
		if want := strings.ToLower(string(r)); folded != want {
			t.Errorf("lower(%q) = %q but strings.ToLower gives %q", string(r), folded, want)
		}
	}
}
