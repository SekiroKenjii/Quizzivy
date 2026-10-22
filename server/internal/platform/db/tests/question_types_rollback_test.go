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

// The migrations of T-R6.1a and T-R6.1b, by the version each one leaves.
const (
	beforeQuestionTypes int64 = 144
	questionTypes       int64 = 145
	selectCount         int64 = 146
	maxWords            int64 = 147
	essaySampleAnswer   int64 = 148
	fixedOrderChoices   int64 = 149
	rubricCriteria      int64 = 150
	matchItems          int64 = 152
	answerBands         int64 = 153
)

// refusal is one Down that must refuse: rolling back to target stops at stays,
// and clear removes the row that blocked it.
type refusal struct {
	name          string
	target, stays int64
	clear         func()
}

func walkRefusals(t *testing.T, conn *sql.DB, dir string, steps []refusal) {
	t.Helper()
	for _, step := range steps {
		if err := goose.DownTo(conn, dir, step.target); err == nil {
			t.Fatalf("the rollback to %d ran while %s exists", step.target, step.name)
		}
		if version, err := goose.GetDBVersion(conn); err != nil || version != step.stays {
			t.Fatalf("version after refusing %s: %d, %v; want %d", step.name, version, err, step.stays)
		}
		step.clear()
	}
}

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

	// Each refusal checks both tables, so each table's value is also left alone
	// in front of it: the bank value goes first, then the frozen one is cleared
	// as the bank one comes back.
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := conn.Exec(query, args...); err != nil {
			t.Fatal(err)
		}
	}
	walkRefusals(t, conn, dir, []refusal{
		{"a fixed tfng member", essaySampleAnswer, fixedOrderChoices, func() {
			exec(`UPDATE app.questions SET context_option_order = 'shuffle' WHERE id = $1`, member)
		}},
		{"an essay's sample answer", essaySampleAnswer - 1, essaySampleAnswer, func() {
			exec(`UPDATE app.questions SET sample_answer = NULL WHERE id = $1`, essay)
		}},
		{"a word limit in both tables", selectCount, maxWords, func() {
			exec(`UPDATE app.questions SET max_words = NULL WHERE id = $1`, limited)
		}},
		{"a frozen word limit alone", selectCount, maxWords, func() {
			exec(`UPDATE app.test_version_questions SET max_words = NULL WHERE id = $1`, frozenLimited)
			exec(`UPDATE app.questions SET max_words = 3 WHERE id = $1`, limited)
		}},
		{"a bank word limit alone", selectCount, maxWords, func() {
			exec(`UPDATE app.questions SET max_words = NULL WHERE id = $1`, limited)
		}},
		{"a choose-two question in both tables", questionTypes, selectCount, func() {
			exec(`UPDATE app.questions SET select_count = NULL WHERE id = $1`, chooseTwo)
		}},
		{"a frozen choose-two question alone", questionTypes, selectCount, func() {
			exec(`UPDATE app.test_version_questions SET select_count = NULL WHERE id = $1`, frozenChoose)
			exec(`UPDATE app.questions SET select_count = 2 WHERE id = $1`, chooseTwo)
		}},
		{"a bank choose-two question alone", questionTypes, selectCount, func() {
			exec(`UPDATE app.questions SET select_count = NULL WHERE id = $1`, chooseTwo)
		}},
		{"a bank question of a new type", beforeQuestionTypes, questionTypes, func() {
			exec(`DELETE FROM app.questions WHERE id = $1`, essay)
		}},
		{"a tfng bank question", beforeQuestionTypes, questionTypes, func() {
			exec(`DELETE FROM app.questions WHERE id = $1`, member)
		}},
		{"a frozen question of a new type", beforeQuestionTypes, questionTypes, func() {
			exec(`DELETE FROM app.test_version_questions WHERE id = $1`, frozen)
		}},
	})
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

// TestTheGradingRollbacksRefuseWhileARowNeedsThem walks the Downs of T-R6.1b
// (QC-R6B-1): a criterion grade without a band stops 00153, a matching question
// in either table stops 00152 before its items go, with or without items, and a criterion in either
// table stops 00150. Each table is left alone in front of its refusal. The
// rollback that then runs must leave 00149's schema, and 00152's Up must apply
// over a matching question written before it.
func TestTheGradingRollbacksRefuseWhileARowNeedsThem(t *testing.T) {
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
	if err := goose.UpTo(conn, dir, fixedOrderChoices); err != nil {
		t.Fatal(err)
	}
	before := schemaSnapshot(t, conn)
	if err := goose.UpTo(conn, dir, answerBands); err != nil {
		t.Fatal(err)
	}

	var author, section, matching, criterion, frozenCriterion string
	withTx(t, conn, func(tx *sql.Tx, f fixture) {
		author = f.adminID
		var attempt, question string
		attempt, question, frozenCriterion = gradedEssay(t, tx, f)
		mustExec(t, tx, `INSERT INTO app.attempt_answer_criteria (attempt_id, question_id, criterion_id, band)
			VALUES ($1, $2, $3, 7.0)`, attempt, question, frozenCriterion)
		var options []string
		matching, options = newMatchingQuestion(t, tx, f.adminID)
		mustExec(t, tx, `INSERT INTO app.question_match_items (question_id, ordinal, text, correct_option_id)
			VALUES ($1, 0, 'Đoạn A', $2)`, matching, options[0])
		essay := newQuestion(t, tx, f.adminID, "essay", "Viết bài luận")
		if err := tx.QueryRow(`INSERT INTO app.question_rubric_criteria (question_id, ordinal, label)
			VALUES ($1, 0, 'Coherence') RETURNING id`, essay).Scan(&criterion); err != nil {
			t.Fatal(err)
		}
		section = newVersion(t, tx, f.adminID)
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	})

	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := conn.Exec(query, args...); err != nil {
			t.Fatal(err)
		}
	}
	var frozenMatching string
	walkRefusals(t, conn, dir, []refusal{
		{"a criterion grade without a band", matchItems, answerBands, func() {
			exec(`DELETE FROM app.attempt_answer_criteria`)
		}},
		{"a bank matching question with an item", rubricCriteria + 1, matchItems, func() {
			exec(`DELETE FROM app.question_match_items WHERE question_id = $1`, matching)
		}},
		{"a bank matching question without items", rubricCriteria + 1, matchItems, func() {
			exec(`DELETE FROM app.questions WHERE id = $1`, matching)
			if err := conn.QueryRow(`INSERT INTO app.test_version_questions
				(test_version_section_id, ordinal, type, prompt, points, match_allow_reuse)
				VALUES ($1, 0, 'matching', 'Nối', 1.0, false) RETURNING id`, section).Scan(&frozenMatching); err != nil {
				t.Fatal(err)
			}
		}},
		{"a frozen matching question", rubricCriteria + 1, matchItems, func() {
			exec(`DELETE FROM app.test_version_questions WHERE id = $1`, frozenMatching)
		}},
		{"rubric criteria in both tables", fixedOrderChoices, rubricCriteria, func() {
			exec(`DELETE FROM app.question_rubric_criteria WHERE id = $1`, criterion)
		}},
		{"a frozen rubric criterion alone", fixedOrderChoices, rubricCriteria, func() {
			exec(`DELETE FROM app.test_version_rubric_criteria WHERE id = $1`, frozenCriterion)
			exec(`INSERT INTO app.question_rubric_criteria (question_id, ordinal, label)
				SELECT id, 0, 'Coherence' FROM app.questions WHERE type = 'essay' AND created_by = $1`, author)
		}},
		{"a bank rubric criterion alone", fixedOrderChoices, rubricCriteria, func() {
			exec(`DELETE FROM app.question_rubric_criteria`)
		}},
	})
	if err := goose.DownTo(conn, dir, fixedOrderChoices); err != nil {
		t.Fatalf("the rollback refused with nothing graded, matched or rubricked: %v", err)
	}
	if after := schemaSnapshot(t, conn); after != before {
		t.Errorf("the rollback left part of T-R6.1b behind\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}

	exec(`INSERT INTO app.questions (type, prompt, points, created_by, owner_id) VALUES ('matching', 'Nối', 1.0, $1, $1)`, author)
	if err := goose.UpTo(conn, dir, answerBands); err != nil {
		t.Fatalf("00152 did not apply over a matching question written before it: %v", err)
	}
	var unset int
	if err := conn.QueryRow(`SELECT count(*) FROM app.questions WHERE type = 'matching' AND match_allow_reuse IS DISTINCT FROM false`).Scan(&unset); err != nil {
		t.Fatal(err)
	}
	if unset != 0 {
		t.Errorf("%d matching questions were not backfilled with match_allow_reuse = false", unset)
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
