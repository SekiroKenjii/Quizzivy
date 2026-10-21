//go:build integration

package db_test

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"

	"quizzivy/internal/platform/db"
)

// The migrations of T-R6.1a: before them, the type values, and the file whose
// Down refuses while an essay has a sample answer.
const (
	beforeQuestionTypes int64 = 144
	questionTypes       int64 = 145
	essaySampleAnswer   int64 = 148
)

func TestTheQuestionTypeRollbackRefusesWhileARowUsesANewType(t *testing.T) {
	if os.Getenv("TEST_DESTRUCTIVE") != "1" {
		t.Skip("TEST_DESTRUCTIVE=1 required for an isolated migration database")
	}
	conn, err := sql.Open("pgx", scratchDatabase(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	goose.SetLogger(goose.NopLogger())
	dir := db.MigrationsDir(t)
	if err := goose.UpTo(conn, dir, beforeQuestionTypes); err != nil {
		t.Fatal(err)
	}
	before := schemaSnapshot(t, conn)
	if err := goose.UpTo(conn, dir, essaySampleAnswer); err != nil {
		t.Fatal(err)
	}

	var essay string
	withTx(t, conn, func(tx *sql.Tx, f fixture) {
		if err := tx.QueryRow(`INSERT INTO app.questions (type, prompt, points, created_by, owner_id, sample_answer)
			VALUES ('essay', 'Viết bài luận', 9.0, $1, $1, 'Bài mẫu') RETURNING id`, f.adminID).Scan(&essay); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	})
	t.Cleanup(func() {
		if _, err := conn.Exec(`DELETE FROM app.questions WHERE id = $1`, essay); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})

	if err := goose.DownTo(conn, dir, essaySampleAnswer-1); err == nil {
		t.Fatal("the sample-answer rollback ran while an essay has a sample answer")
	}
	if _, err := conn.Exec(`UPDATE app.questions SET sample_answer = NULL WHERE id = $1`, essay); err != nil {
		t.Fatal(err)
	}
	if err := goose.DownTo(conn, dir, beforeQuestionTypes); err == nil {
		t.Fatal("the type rollback ran while an essay exists")
	}
	if version, err := goose.GetDBVersion(conn); err != nil || version != questionTypes {
		t.Fatalf("version after the refused rollback: %d, %v; want %d", version, err, questionTypes)
	}

	if _, err := conn.Exec(`DELETE FROM app.questions WHERE id = $1`, essay); err != nil {
		t.Fatal(err)
	}
	if err := goose.DownTo(conn, dir, beforeQuestionTypes); err != nil {
		t.Fatalf("the rollback refused with no row of a new type: %v", err)
	}
	var labels string
	if err := conn.QueryRow(`SELECT array_to_string(enum_range(NULL::app.question_type), ',')`).Scan(&labels); err != nil {
		t.Fatal(err)
	}
	if want := "single_choice,multiple_choice,true_false,fill_blank,short_answer,tfng,matching,essay"; labels != want {
		t.Errorf("question_type after the rollback = %s, want %s: the Down leaves the values (D-62)", labels, want)
	}
	if err := goose.UpTo(conn, dir, essaySampleAnswer); err != nil {
		t.Fatalf("up after the rollback found the values present and failed: %v", err)
	}
	if err := goose.DownTo(conn, dir, beforeQuestionTypes); err != nil {
		t.Fatal(err)
	}
	if after := schemaSnapshot(t, conn); after == before {
		t.Error("the snapshot is unchanged although the type keeps three values; the comparison below proves nothing")
	} else if stripQuestionType(before) != stripQuestionType(after) {
		t.Errorf("the rollback left more than the three enum values behind\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}
}

func stripQuestionType(snapshot string) string {
	var kept []string
	for _, line := range strings.Split(snapshot, "\n") {
		if !strings.HasPrefix(line, "type question_type ") {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}
