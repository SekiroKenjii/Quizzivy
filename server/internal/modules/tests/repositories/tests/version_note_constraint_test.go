//go:build integration

package repositories_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"quizzivy/internal/platform/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestTestVersionsKeepTheOldInsertValidAndHoldTheNoteToTwoHundredCharacters(t *testing.T) {
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
			t.Errorf("note constraint rollback: %v", err)
			return
		}
		var remaining int
		if err := pool.QueryRow(context.Background(),
			`SELECT (SELECT count(*) FROM app.users WHERE id=$1)+(SELECT count(*) FROM app.tests WHERE created_by=$1)+(SELECT count(*) FROM app.test_versions WHERE published_by=$1)`,
			owner).Scan(&remaining); err != nil || remaining != 0 {
			t.Errorf("note constraint owned rows left=%d %v", remaining, err)
		}
	})
	if _, err := tx.Exec(ctx, `INSERT INTO app.users(id,email,full_name,role_id) VALUES($1,$2,'Note teacher',(SELECT id FROM app.roles WHERE builtin_key='teacher'))`, owner, owner+"@example.test"); err != nil {
		t.Fatal(err)
	}
	var testID string
	if err := tx.QueryRow(ctx, `INSERT INTO app.tests(title, created_by, owner_id) VALUES('Note', $1, $1) RETURNING id::text`, owner).Scan(&testID); err != nil {
		t.Fatal(err)
	}

	var note *string
	if err := tx.QueryRow(ctx,
		`INSERT INTO app.test_versions (test_id, version, total_points, published_at, published_by, delivery_version)
		 VALUES ($1, 1, 1, now(), $2, 'group_v1') RETURNING change_note`, testID, owner).Scan(&note); err != nil {
		t.Fatalf("the previous release's insert: %v", err)
	}
	if note != nil {
		t.Fatalf("a version inserted without a note reads %q, want NULL", *note)
	}

	insert := func(version int, text string) error {
		sp, err := tx.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		_, err = sp.Exec(ctx,
			`INSERT INTO app.test_versions (test_id, version, total_points, published_at, published_by, delivery_version, change_note)
			 VALUES ($1, $2, 1, now(), $3, 'group_v1', $4)`, testID, version, owner, text)
		if err != nil {
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
	for _, c := range []struct {
		name    string
		version int
		text    string
		refused bool
	}{
		{"one character", 2, "đ", false},
		{"two hundred characters", 3, strings.Repeat("đ", 200), false},
		{"two hundred and one characters", 4, strings.Repeat("đ", 201), true},
		{"empty", 5, "", true},
	} {
		err := insert(c.version, c.text)
		switch {
		case c.refused && !db.IsCheckViolation(err, "test_versions_change_note_check"):
			t.Errorf("%s: err = %v, want test_versions_change_note_check", c.name, err)
		case !c.refused && err != nil:
			t.Errorf("%s: %v", c.name, err)
		}
	}

	var validated bool
	if err := tx.QueryRow(ctx, `SELECT convalidated FROM pg_constraint WHERE conname = 'test_versions_change_note_check'`).Scan(&validated); err != nil {
		t.Fatalf("test_versions_change_note_check: %v", err)
	}
	if !validated {
		t.Error("test_versions_change_note_check is not validated")
	}
}
