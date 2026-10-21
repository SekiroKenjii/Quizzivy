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

// The migrations of T-R6.1a, by the version each one leaves, and T-R6.1b's
// bands.
const (
	beforeQuestionTypes int64 = 144
	questionTypes       int64 = 145
	selectCount         int64 = 146
	maxWords            int64 = 147
	essaySampleAnswer   int64 = 148
	fixedOrderChoices   int64 = 149
	answerBands         int64 = 153
)

// TestTheQuestionTypeRollbacksRefuseWhileARowNeedsThem walks every Down of
// T-R6.1a past each row that blocks it, and keeps rows each Down must let
// through (a short answer's sample answer, a single choice's fixed order): the
// version each refusal leaves shows the Down before it did not over-refuse.
func TestTheQuestionTypeRollbacksRefuseWhileARowNeedsThem(t *testing.T) {
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
	if err := goose.UpTo(conn, dir, fixedOrderChoices); err != nil {
		t.Fatal(err)
	}

	var essay, chooseTwo, limited, member, frozen, frozenChoose, frozenLimited string
	withTx(t, conn, func(tx *sql.Tx, f fixture) {
		scan := func(dest *string, insert string, args ...any) {
			t.Helper()
			if err := tx.QueryRow(insert, args...).Scan(dest); err != nil {
				t.Fatal(err)
			}
		}
		scan(&essay, `INSERT INTO app.questions (type, prompt, points, created_by, owner_id, sample_answer)
			VALUES ('essay', 'Viết bài luận', 9.0, $1, $1, 'Bài mẫu') RETURNING id`, f.adminID)
		scan(&chooseTwo, `INSERT INTO app.questions (type, prompt, points, created_by, owner_id, select_count)
			VALUES ('multiple_choice', 'Chọn HAI đáp án', 2.0, $1, $1, 2) RETURNING id`, f.adminID)
		scan(&member, `INSERT INTO app.questions
			(type, prompt, points, created_by, owner_id, context_group_id, context_ordinal, context_option_order)
			VALUES ('tfng', 'Đúng, sai hay không có', 1.0, $1, $1, $2, 0, 'fixed') RETURNING id`,
			f.adminID, newGroup(t, tx, f.adminID, nil))
		scan(&limited, `INSERT INTO app.questions (type, prompt, points, created_by, owner_id, max_words)
			VALUES ('short_answer', 'Không quá BA từ', 1.0, $1, $1, 3) RETURNING id`, f.adminID)
		section := newVersion(t, tx, f.adminID)
		scan(&frozen, `INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, points)
			VALUES ($1, 0, 'tfng', 'Câu đã chốt', 1.0) RETURNING id`, section)
		scan(&frozenChoose, `INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, points, select_count)
			VALUES ($1, 1, 'multiple_choice', 'Chọn HAI đáp án', 2.0, 2) RETURNING id`, section)
		scan(&frozenLimited, `INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, points, max_words)
			VALUES ($1, 2, 'short_answer', 'Không quá BA từ', 1.0, 3) RETURNING id`, section)
		mustExec(t, tx, `INSERT INTO app.questions (type, prompt, points, created_by, owner_id, sample_answer)
			VALUES ('short_answer', 'Viết câu trả lời', 1.0, $1, $1, 'Đáp án mẫu')`, f.adminID)
		mustExec(t, tx, `INSERT INTO app.questions
			(type, prompt, points, created_by, owner_id, context_group_id, context_ordinal, context_option_order)
			VALUES ('single_choice', 'Chọn một', 1.0, $1, $1, $2, 0, 'fixed')`, f.adminID, newGroup(t, tx, f.adminID, nil))
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	})

	for _, step := range []struct {
		name          string
		target, stays int64
		clear         string
		id            *string
	}{
		{"a fixed tfng member", essaySampleAnswer, fixedOrderChoices,
			`UPDATE app.questions SET context_option_order = 'shuffle' WHERE id = $1`, &member},
		{"an essay's sample answer", essaySampleAnswer - 1, essaySampleAnswer,
			`UPDATE app.questions SET sample_answer = NULL WHERE id = $1`, &essay},
		{"a short answer with a word limit", selectCount, maxWords,
			`UPDATE app.questions SET max_words = NULL WHERE id = $1`, &limited},
		{"a frozen short answer with a word limit", selectCount, maxWords,
			`UPDATE app.test_version_questions SET max_words = NULL WHERE id = $1`, &frozenLimited},
		{"a choose-two question", questionTypes, selectCount,
			`UPDATE app.questions SET select_count = NULL WHERE id = $1`, &chooseTwo},
		{"a frozen choose-two question", questionTypes, selectCount,
			`UPDATE app.test_version_questions SET select_count = NULL WHERE id = $1`, &frozenChoose},
		{"a bank question of a new type", beforeQuestionTypes, questionTypes,
			`DELETE FROM app.questions WHERE id = $1`, &essay},
		{"a tfng bank question", beforeQuestionTypes, questionTypes,
			`DELETE FROM app.questions WHERE id = $1`, &member},
		{"a frozen question of a new type", beforeQuestionTypes, questionTypes,
			`DELETE FROM app.test_version_questions WHERE id = $1`, &frozen},
	} {
		if err := goose.DownTo(conn, dir, step.target); err == nil {
			t.Fatalf("the rollback to %d ran while %s exists", step.target, step.name)
		}
		if version, err := goose.GetDBVersion(conn); err != nil || version != step.stays {
			t.Fatalf("version after refusing %s: %d, %v; want %d", step.name, version, err, step.stays)
		}
		if _, err := conn.Exec(step.clear, *step.id); err != nil {
			t.Fatal(err)
		}
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
	if err := goose.UpTo(conn, dir, fixedOrderChoices); err != nil {
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

func TestTheBandRollbackRefusesWhileABandExists(t *testing.T) {
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
	if err := goose.UpTo(conn, dir, answerBands); err != nil {
		t.Fatal(err)
	}
	var attempt, question string
	withTx(t, conn, func(tx *sql.Tx, f fixture) {
		attempt, question, _ = gradedEssay(t, tx, f)
		mustExec(t, tx, `UPDATE app.attempt_answers SET band = 6.5, manual_score = 6.50, graded_at = now()
			WHERE attempt_id = $1 AND question_id = $2`, attempt, question)
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	})
	before := schemaSnapshot(t, conn)
	if err := goose.DownTo(conn, dir, answerBands-1); err == nil {
		t.Fatal("the band rollback discarded a graded band")
	}
	if schemaSnapshot(t, conn) != before {
		t.Fatal("the refused rollback changed the schema")
	}
	if _, err := conn.Exec(`UPDATE app.attempt_answers SET band = NULL WHERE attempt_id = $1 AND question_id = $2`,
		attempt, question); err != nil {
		t.Fatal(err)
	}
	if err := goose.DownTo(conn, dir, answerBands-1); err != nil {
		t.Fatalf("the rollback refused with no band: %v", err)
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
