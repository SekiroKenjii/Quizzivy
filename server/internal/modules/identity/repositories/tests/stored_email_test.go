//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const storedEmailHeading = "## Stored emails the API cannot print"

func detectionQuery(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "..", "docs", "setup", "operations.md"))
	if err != nil {
		t.Fatalf("read docs/setup/operations.md: %v", err)
	}
	_, section, found := strings.Cut(string(raw), storedEmailHeading+"\n")
	if !found {
		t.Fatalf("docs/setup/operations.md has no %q section", storedEmailHeading)
	}
	section, _, _ = strings.Cut(section, "\n## ")
	_, block, found := strings.Cut(section, "```sql\n")
	if !found {
		t.Fatalf("the %q section has no sql block", storedEmailHeading)
	}
	query, _, found := strings.Cut(block, "```")
	if !found {
		t.Fatalf("the %q section's sql block is not closed", storedEmailHeading)
	}
	return query
}

func storedEmailCorpus() []string {
	var locals []string
	for c := rune(1); c < 128; c++ {
		s := string(c)
		locals = append(locals, s, "a"+s, s+"a", "a"+s+"b", "a"+s+s+"b")
		for d := rune(1); d < 128; d++ {
			locals = append(locals, "a"+s+string(d)+"b")
		}
	}
	for r := rune(0x80); r < 0x3000; r++ {
		if r >= 0xd800 && r < 0xe000 {
			continue
		}
		s := string(r)
		locals = append(locals, "a"+s+"b", s+"a", "a"+s)
	}
	locals = append(locals,
		"a", "a.b", "a.b.c", "a+tag", "a-b", "a_b", "a'b", "hoc.sinh.é", "bücher", "nguyễn.văn.a",
		"a@b", "a@b@c", `"a"`, `a\b`, "a b", " a", "a ", ".a", "a.", "a..b", ".", "..", "a...b", "a.b.",
		"qc tp", "a\tb", "a\nb", "a\x7fb", "a\x1fb", "a\x20b",
	)
	emails := make([]string, len(locals))
	for i, local := range locals {
		emails[i] = fmt.Sprintf("%s@h%d.example", local, i)
	}
	return emails
}

func printsBack(email string) bool {
	parsed, err := mail.ParseAddress(email)
	return err == nil && parsed.Address == email
}

func TestTheStoredEmailQueryFindsExactlyTheRowsTheAPICannotPrintBack(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	query := detectionQuery(t)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("fixture rollback: %v", err)
		}
	})

	corpus := storedEmailCorpus()
	if _, err := tx.Exec(ctx, `INSERT INTO app.users(email, full_name, role_id)
		SELECT e, 'Fixture', (SELECT id FROM app.roles WHERE builtin_key = 'student') FROM unnest($1::text[]) AS e`, corpus); err != nil {
		t.Fatal(err)
	}
	rows, err := tx.Query(ctx, query)
	if err != nil {
		t.Fatalf("the documented query: %v", err)
	}
	found := map[string]bool{}
	for rows.Next() {
		var id, email string
		if err := rows.Scan(&id, &email); err != nil {
			t.Fatal(err)
		}
		found[email] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	var bad, good int
	for _, email := range corpus {
		if printsBack(email) {
			good++
		} else {
			bad++
		}
		if want := !printsBack(email); found[email] != want {
			t.Errorf("the query returns %q = %v, but mail.ParseAddress says the API cannot print it back = %v", email, found[email], want)
		}
	}
	if bad == 0 || good == 0 {
		t.Fatalf("the corpus holds %d unprintable and %d printable addresses, so it proves nothing", bad, good)
	}
}

func TestTheStoredEmailQueryHasNoUnicodeClauseBecauseNothingBeyondASCIIIsRefused(t *testing.T) {
	var refused []rune
	for r := rune(0x80); r < 0x3000; r++ {
		if r >= 0xd800 && r < 0xe000 {
			continue
		}
		for _, email := range []string{"a" + string(r) + "b@h.example", string(r) + "a@h.example", "a" + string(r) + "@h.example"} {
			if !printsBack(email) && !slices.Contains(refused, r) {
				refused = append(refused, r)
			}
		}
	}
	if len(refused) > 0 {
		t.Fatalf("mail.ParseAddress refuses %d code points beyond ASCII, first %U; the query in docs/setup/operations.md needs a clause for them", len(refused), refused[0])
	}
}
