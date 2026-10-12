//go:build integration

package repositories_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"quizzivy/internal/platform/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func altTransaction(t *testing.T) (context.Context, pgx.Tx, string) {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	owner := uuid.NewString()
	t.Cleanup(func() {
		if err := tx.Rollback(context.Background()); err != nil {
			t.Errorf("alt text constraint rollback: %v", err)
			return
		}
		var remaining int
		if err := pool.QueryRow(context.Background(),
			`SELECT (SELECT count(*) FROM app.users WHERE id=$1)+(SELECT count(*) FROM app.questions WHERE created_by=$1)+(SELECT count(*) FROM app.tests WHERE created_by=$1)`,
			owner).Scan(&remaining); err != nil || remaining != 0 {
			t.Errorf("alt text constraint owned rows left=%d %v", remaining, err)
		}
	})
	if _, err := tx.Exec(ctx, `INSERT INTO app.users(id,email,full_name,role_id) VALUES($1,$2,'Alt teacher',(SELECT id FROM app.roles WHERE builtin_key='teacher'))`, owner, owner+"@example.test"); err != nil {
		t.Fatal(err)
	}
	return ctx, tx, owner
}

func insertCase(ctx context.Context, t *testing.T, tx pgx.Tx, query string, args ...any) error {
	t.Helper()
	sp, err := tx.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sp.Exec(ctx, query, args...); err != nil {
		if rollbackErr := sp.Rollback(ctx); rollbackErr != nil {
			t.Fatalf("savepoint rollback: %v", rollbackErr)
		}
		return err
	}
	if err := sp.Commit(ctx); err != nil {
		t.Fatalf("savepoint release: %v", err)
	}
	return nil
}

func TestTheBankColumnKeepsTheOldInsertValidAndHoldsAltTextToAThousandCharacters(t *testing.T) {
	ctx, tx, owner := altTransaction(t)

	var alt *string
	if err := tx.QueryRow(ctx,
		`INSERT INTO app.questions(type, prompt, points, created_by, owner_id) VALUES('short_answer', 'The previous release', 1, $1, $1) RETURNING media_alt`,
		owner).Scan(&alt); err != nil {
		t.Fatalf("the previous release's insert: %v", err)
	}
	if alt != nil {
		t.Fatalf("a question inserted without alt text reads %q, want NULL", *alt)
	}

	for _, c := range []struct {
		name    string
		text    string
		refused bool
	}{
		{"one character", "đ", false},
		{"a thousand characters", strings.Repeat("đ", 1000), false},
		{"a thousand and one characters", strings.Repeat("đ", 1001), true},
		{"empty", "", true},
	} {
		err := insertCase(ctx, t, tx,
			`INSERT INTO app.questions(type, prompt, points, created_by, owner_id, media_alt) VALUES('short_answer', 'x', 1, $1, $1, $2)`, owner, c.text)
		switch {
		case c.refused && !db.IsCheckViolation(err, "questions_media_alt_check"):
			t.Errorf("%s: err = %v, want questions_media_alt_check", c.name, err)
		case !c.refused && err != nil:
			t.Errorf("%s: %v", c.name, err)
		}
	}

	var validated bool
	if err := tx.QueryRow(ctx, `SELECT convalidated FROM pg_constraint WHERE conname = 'questions_media_alt_check'`).Scan(&validated); err != nil || !validated {
		t.Errorf("questions_media_alt_check validated = %v, err = %v", validated, err)
	}
}

func TestTheFrozenColumnKeepsTheOldInsertValidAndHoldsAltTextToAThousandCharacters(t *testing.T) {
	ctx, tx, owner := altTransaction(t)

	var testID, versionID, sectionID string
	if err := tx.QueryRow(ctx, `INSERT INTO app.tests(title, created_by, owner_id) VALUES('Alt', $1, $1) RETURNING id::text`, owner).Scan(&testID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx,
		`INSERT INTO app.test_versions (test_id, version, total_points, published_at, published_by, delivery_version)
		 VALUES ($1, 1, 1, now(), $2, 'group_v1') RETURNING id::text`, testID, owner).Scan(&versionID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO app.test_version_sections(test_version_id, ordinal, title) VALUES($1, 0, 'Phần 1') RETURNING id::text`, versionID).Scan(&sectionID); err != nil {
		t.Fatal(err)
	}

	var alt *string
	if err := tx.QueryRow(ctx,
		`INSERT INTO app.test_version_questions(test_version_section_id, ordinal, type, prompt, points) VALUES($1, 0, 'short_answer', 'The previous release', 1) RETURNING media_alt`,
		sectionID).Scan(&alt); err != nil {
		t.Fatalf("the previous release's insert: %v", err)
	}
	if alt != nil {
		t.Fatalf("a frozen question inserted without alt text reads %q, want NULL", *alt)
	}

	for i, c := range []struct {
		name    string
		text    string
		refused bool
	}{
		{"one character", "đ", false},
		{"a thousand characters", strings.Repeat("đ", 1000), false},
		{"a thousand and one characters", strings.Repeat("đ", 1001), true},
		{"empty", "", true},
	} {
		err := insertCase(ctx, t, tx,
			`INSERT INTO app.test_version_questions(test_version_section_id, ordinal, type, prompt, points, media_alt) VALUES($1, $2, 'short_answer', 'x', 1, $3)`,
			sectionID, i+1, c.text)
		switch {
		case c.refused && !db.IsCheckViolation(err, "test_version_questions_media_alt_check"):
			t.Errorf("%s: err = %v, want test_version_questions_media_alt_check", c.name, err)
		case !c.refused && err != nil:
			t.Errorf("%s: %v", c.name, err)
		}
	}

	var validated bool
	if err := tx.QueryRow(ctx, `SELECT convalidated FROM pg_constraint WHERE conname = 'test_version_questions_media_alt_check'`).Scan(&validated); err != nil || !validated {
		t.Errorf("test_version_questions_media_alt_check validated = %v, err = %v", validated, err)
	}
}
